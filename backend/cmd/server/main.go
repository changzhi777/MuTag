// MuTag 活码服务入口：配置加载 → 数据库（SQLite 开发 / PG 上线）→ 自动迁移 → HTTP 服务 → 优雅退出。
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/api"
	"mutag/backend/internal/cache"
	"mutag/backend/internal/config"
	"mutag/backend/internal/db"
	"mutag/backend/internal/h5"
	"mutag/backend/internal/repo"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 存储形态：DATABASE_URL 有值 → PG16；否则 SQLite（零外部依赖）
	dialect := db.DialectSQLite
	var database *sql.DB
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

	// 启动即自动迁移（幂等，方言独立版本线）
	if err := db.Migrate(ctx, database, dialect); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store := repo.NewStore(database)
	api.SeedAdmin(store, cfg.AdminSeed) // 0007：admins 空且 ADMIN_SEED 有值时播种首个管理员

	renderer, err := h5.NewRenderer()
	if err != nil {
		log.Fatalf("h5 renderer: %v", err)
	}

	deps := api.Deps{
		DB:            database,
		Store:         store,
		Cache:         cache.New(60 * time.Second),
		Limiter:       cache.NewLimiter(cfg.RateBurst, cfg.RatePerSec),
		H5:            renderer,
		AdminPassword: cfg.AdminPassword,
		BaseURL:       cfg.BaseURL,
		WechatAppID:   cfg.WechatAppID,
		WechatSecret:  cfg.WechatSecret,
		TokenSecret:   cfg.TokenSecret,
		WeappJumpURL:  cfg.WeappJumpURL,
	}

	gin.SetMode(gin.ReleaseMode)
	router := api.NewRouter(deps)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("MuTag 活码服务启动 %s (db=%s, base=%s)", cfg.ListenAddr, cfg.SQLitePath, cfg.BaseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	// 优雅退出：SIGINT/SIGTERM → 10s 排空
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("服务已退出")
}
