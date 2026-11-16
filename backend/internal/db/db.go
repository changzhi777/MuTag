// Package db 数据库连接与自动迁移（embed SQL + 轻量迁移器）。
// 双形态：SQLite（modernc 纯 Go，开发/验收零外部依赖）与 PG（pgx stdlib，上线）。
// 方言隔离原则：迁移文件按方言各自维护（*.pg.* 中缀），repo 层 SQL 双方言同一份
// （占位符统一 ? + 时间戳应用层传值）；PG 走 pgx 简单协议以支持 ? 占位符。
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	_ "modernc.org/sqlite" // 注册 "sqlite" database/sql driver
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Dialect 数据库方言：迁移文件按方言选择。
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite" // 文件：0001_init.up.sql（无 .pg. 中缀）
	DialectPostgres Dialect = "pg"     // 文件：0001_init.pg.up.sql
)

// OpenSQLite 打开（必要时创建）SQLite 库文件，启用 WAL 与外键。
func OpenSQLite(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite 单写者：连接池收口为 1 写 + N 读由 WAL 承担，简单起见限 1
	database.SetMaxOpenConns(1)

	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return database, nil
}

// OpenPostgres 打开 PG 连接（pgx + rebind 驱动包装）。
// pgx 全链路（含 simple protocol）只认 $n 占位符；repo 层统一 ? 风格，
// 故经 rebind_driver 包装层自动转换（? → $1..$n），双方言同一份 SQL。
// DefaultQueryExecMode 强制 simple protocol：无 prepared statement——
// transaction pooler（Supavisor 6543）下物理连接复用不会残留 42P05。
func OpenPostgres(dsn string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pg dsn: %w", err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	database := OpenWithRebind(stdlib.GetConnector(*cfg))
	database.SetMaxOpenConns(10)

	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return database, nil
}

// schemaMigrationsDDL 版本表建表语句按方言（默认值语法不同）。
var schemaMigrationsDDL = map[Dialect]string{
	DialectSQLite: `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
	)`,
	DialectPostgres: `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (now()::text)
	)`,
}

// selectUpMigrations 按方言筛选未应用的 .up.sql（版本即文件名，双方言独立版本线）。
func selectUpMigrations(entries []os.DirEntry, dialect Dialect) ([]string, error) {
	var ups []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		isPG := strings.Contains(name, ".pg.")
		if (dialect == DialectPostgres) != isPG {
			continue
		}
		ups = append(ups, name)
	}
	sort.Strings(ups)
	return ups, nil
}

// Migrate 执行所选方言的全部未应用 .up.sql（幂等，启动时自动调用）。
func Migrate(ctx context.Context, database *sql.DB, dialect Dialect) error {
	ddl, ok := schemaMigrationsDDL[dialect]
	if !ok {
		return fmt.Errorf("unknown dialect %q", dialect)
	}
	if _, err := database.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	ups, err := selectUpMigrations(entries, dialect)
	if err != nil {
		return err
	}

	for _, name := range ups {
		var applied bool
		if err := database.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = ?)`, name).
			Scan(&applied); err != nil {
			return fmt.Errorf("check version %s: %w", name, err)
		}
		if applied {
			continue
		}

		sqlBytes, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		// 多语句直发：SQLite modernc 支持；PG 走简单协议（多语句文本同发）
		if _, err := database.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := database.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
		log.Printf("migration applied (%s): %s", dialect, name)
	}
	return nil
}
