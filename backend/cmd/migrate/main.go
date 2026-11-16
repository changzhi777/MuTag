// MuTag 独立迁移执行器：部署期在应用启动前显式跑迁移（幂等）。
// 用法：
//
//	SQLite（默认）：DATABASE_URL 为空 → SQLITE_PATH
//	PG 上线：       DATABASE_URL=postgres://... go run ./cmd/migrate
package main

import (
	"context"
	"database/sql"
	"log"

	"mutag/backend/internal/config"
	"mutag/backend/internal/db"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	var database *sql.DB
	dialect := db.DialectSQLite
	var err error
	if cfg.DatabaseURL != "" {
		dialect = db.DialectPostgres
		database, err = db.OpenPostgres(cfg.DatabaseURL)
	} else {
		database, err = db.OpenSQLite(cfg.SQLitePath)
	}
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(ctx, database, dialect); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("迁移完成（dialect=%s）", dialect)
}
