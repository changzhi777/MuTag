# MuTag 部署

## 形态说明

| 形态 | 存储 | 缓存 | 用途 |
|---|---|---|---|
| **开发/验收（当前）** | SQLite（modernc 纯 Go 驱动，`backend/data/mutag.db`） | 进程内 TTL 缓存（`internal/cache`） | 零外部依赖，`go run` 直跑 |
| **上线（M2+）** | PG16 + Redis7（本目录 compose） | Redis 60s TTL | 118/PVE CT 部署 |

切换点收敛在三处：连接层（`internal/db` OpenSQLite ↔ pgx pool）、迁移方言（`internal/db/migrations/` 各自维护）、缓存注入（`cache.TTLCache` ↔ go-redis）。

## 本地验收（SQLite）

```bash
bash deploy/verify.sh    # 一键：重置库 → 编译 → 起服务(:18080) → seed 74 条 → 七连检
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

## PG 上线形态（M2+ 启用）

```bash
cd deploy && docker compose up -d      # PG16(:5433) + Redis7(:6380)
```

部署目标为家庭实验室 PVE CT（deploy-ct151 模式），后端多阶段构建见 `backend/Dockerfile`（当前构建目标 SQLite 形态；PG 形态切回 pgx 连接层后同 Dockerfile 流程）。
