package db

// 迁移器方言筛选测试：SQLite/PG 双版本线互不污染（切换正确性关键路径）。

import (
	"strings"
	"testing"
)

func TestSelectUpMigrationsByDialect(t *testing.T) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}

	sqliteUps, err := selectUpMigrations(entries, DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	pgUps, err := selectUpMigrations(entries, DialectPostgres)
	if err != nil {
		t.Fatal(err)
	}

	if len(sqliteUps) != 3 || len(pgUps) != 3 {
		t.Fatalf("各方言应各 3 个迁移: sqlite=%d pg=%d", len(sqliteUps), len(pgUps))
	}
	for _, n := range pgUps {
		if !strings.Contains(n, ".pg.") {
			t.Fatalf("PG 版本线混入 SQLite 文件: %s", n)
		}
	}
	for _, n := range sqliteUps {
		if strings.Contains(n, ".pg.") {
			t.Fatalf("SQLite 版本线混入 PG 文件: %s", n)
		}
	}
}
