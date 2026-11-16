# MuTag 部署

## 形态说明（三档）

| 形态 | 存储 | 缓存 | 用途 |
|---|---|---|---|
| **开发/验收（当前）** | SQLite（modernc 纯 Go 驱动，`backend/data/mutag.db`） | 进程内 TTL 缓存（`internal/cache`） | 零外部依赖，`go run` 直跑 |
| **上线·推荐（2026-11-16 裁决）** | **Supabase 托管 PostgreSQL 17** | 进程内 TTL（Supavisor 连接池由 Supabase 侧承担） | 免运维、免费层 500MB、每日备份 |
| **上线·备选** | 自建 PG17（本目录 compose）+ Redis7 | Redis 60s TTL | 私有化 / 离线场景（PVE CT） |

切换点收敛在三处：连接层（`internal/db` OpenSQLite ↔ OpenPostgres，pgx 简单协议）、迁移方言（`internal/db/migrations/` 按 `.pg.` 中缀各自维护独立版本线）、缓存注入（`cache.TTLCache` ↔ go-redis 预留）。

**形态选择只需一个变量**：`DATABASE_URL` 有值 → PG（Supabase/自建）；空 → SQLite。

## Supabase 接入（上线推荐，已实证 ✅ 2026-11-17）

全套 e2e（8 组含财务/网格/限流链路）已在 Supabase PG17 真实跑绿；期间落地的适配：占位符 rebind 驱动包装（`?`→`$n`）、simple protocol（免疫 transaction pooler 的 42P05）、`RETURNING` 替代 LastInsertId、全表 INSERT 显式时间戳。

1. supabase.com → New Project（区域 Singapore，免费层 500MB 够 MuTag 量级）
2. Project Settings → Database → Connection string → **Session pooler**（5432，事务语义最稳）
3. 先跑迁移再起服务（`cmd/migrate` 幂等，可重复执行）：

```bash
export DATABASE_URL='postgres://postgres.<ref>:<密码>@aws-0-<region>.pooler.supabase.com:5432/postgres?sslmode=require'
cd backend && go run ./cmd/migrate     # 三迁移落库 + 幂等重跑校验
go run ./cmd/server                    # 同 DATABASE_URL 自动识别 PG 形态
```

要点：
- **sslmode=require 必带**（Supabase 强制 TLS）
- pgx 走**简单协议**（OpenPostgres 自动追加 `default_query_exec_mode=simple_protocol`）——规避 Supavisor pooler 的 prepared statements 坑，repo 层 `?` 占位符双方言通用
- confirm/rollback 的显式事务在 pooler 下语义保持（单事务单连接）
- 双方言实证：`MUTAG_TEST_PG_DSN="$DATABASE_URL" go test ./internal/api/ -v` 全套 e2e 跑在真 PG 上

## 本地验收（SQLite）

```bash
bash deploy/verify.sh    # 一键：重置库 → 编译 → 起服务(:18080) → seed 74 条 → 15 连检
```

手工流程：

```bash
cd backend
go run ./cmd/server                    # 启动即自动迁移，监听 :8080
python3 scripts/xlsx_to_json.py "../固定资产需打标签.xlsx" scripts/seed/data.json
go run ./scripts/seed                  # 一物一码导入
curl -i localhost:8080/t/MT-2026-000001
```

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| LISTEN_ADDR | :8080 | HTTP 监听 |
| SQLITE_PATH | data/mutag.db | SQLite 库文件（相对 backend/） |
| BASE_URL | http://localhost:8080 | 活码对外域名（生产 https://mu.qingmu.cn） |
| DATABASE_URL | （空） | PG 连接串（Supabase/自建），有值即上线形态 |
| ADMIN_PASSWORD | （空） | 管理后台密码（空=免鉴权） |
| RATE_LIMIT_BURST / RATE_LIMIT_PER_SEC | 30 / 5 | 扫码限流桶（≤0 关闭） |

## 自建 PG17 上线形态（私有化备选）

```bash
cd deploy && docker compose up -d      # PG17(:5433) + Redis7(:6380)
```

后端多阶段构建见 `backend/Dockerfile`；目标环境家庭实验室 PVE CT（deploy-ct151 模式）。
