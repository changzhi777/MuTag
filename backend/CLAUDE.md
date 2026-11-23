[根目录](../CLAUDE.md) > **backend**

# backend · MuTag 活码服务 — 模块文档

> 更新：2026-11-18（init-architect V1.16 首建）｜ Go 1.26 + Gin + SQLite（modernc）/ Supabase PG17 双形态
> 根上下文：[../CLAUDE.md](../CLAUDE.md) · 唯一事实来源：[../MuTag-项目设计方案.md](../MuTag-项目设计方案.md)

---

## 变更记录 (Changelog)

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2026-11-16 | — | M1 骨架（活码 302 + H5 + 四表自迁移）→ M4 财务三表与两段式导入 → M2 员工端登录/快捷操作；V1.7 PG 方言准备 → V1.8 pgx 激活 + cmd/migrate → V1.9 Supabase 真 PG 全套 e2e 实证（rebind 驱动/simple protocol/RETURNING） |
| 2026-11-17 | — | V1.11 扫码统计 `/tags/scan-stats` → V1.12 `/api/admin/config` 下发 BASE_URL → V1.14 服务端 QR 端点 `/qr/:urlCode` 上线（skip2/go-qrcode，106 容器实证 + 公网 200 实测；go.sum 欠账清偿）→ V1.15 cleanup 域（0004 迁移 + scan/execute/restore 三步流）+ dedupe 导入去重（precheck/dedup 默认 true） |
| 2026-11-18 | V1.0 | 🆕 本模块 CLAUDE.md 首建（init-architect V1.16）：接口全景 / 双形态存储 / 21 测试函数清点入档；qr-logo（`internal/qr` withLogo 升级）登记为待办未实施 |

---

## 模块职责

MuTag 全部服务端逻辑的唯一载体：活码 302 分流（码不变内容改）、H5 只读详情 SSR、小程序/管理后台 JSON API、服务端 QR 码图渲染、SQLite/PG 双形态存储与方言迁移、进程内 TTL 缓存与 IP 令牌桶限流。管理后台 SPA 构建产物经 `internal/adminui` embed 进单二进制。

## 入口与启动

| 入口 | 说明 |
|------|------|
| `cmd/server/main.go` | 主服务：config.Load → OpenSQLite/OpenPostgres（`DATABASE_URL` 有值切 PG）→ `db.Migrate` 启动即幂等迁移 → `api.NewRouter` → :8080，SIGTERM 10s 优雅退出 |
| `cmd/migrate/main.go` | 独立迁移执行器（部署期先迁移再起服务；双方言版本线） |
| `scripts/seed/main.go` | 一物一码导入（消费 `scripts/seed/data.json`，由 `scripts/xlsx_to_json.py` 从真实 xlsx 生成） |
| `Dockerfile` | 多阶段：golang:1.26-alpine 构建 CGO_ENABLED=0 单二进制 → alpine:3.20 非 root（uid 10001）运行，EXPOSE 8080 |

```bash
# 本地开发（SQLite 零依赖）
go run ./cmd/server                      # 启动即自动迁移
# PG 形态（Supabase）
DATABASE_URL='postgres://...?sslmode=require' go run ./cmd/migrate && go run ./cmd/server
# 沙箱离线构建
GOFLAGS="-mod=mod -buildvcs=false" GOPROXY=off GOSUMDB=off go build ./cmd/server
```

## 对外接口（路由全景，`internal/api/router.go`）

### 公开（扫码面，限流同桶——纪律 4）

| 方法路径 | 行为 |
|---|---|
| GET/HEAD `/healthz` | 健康探针 `{"status":"ok"}` |
| GET `/` | H5 落地页；NoRoute 兜底 |
| GET/HEAD `/t/:urlCode` | 活码分流：active → 302（H5 或 `WEAPP_JUMP_URL`）；void/未知 → 200「已作废」/404；HEAD 兼容扫码枪 |
| GET `/h5/t/:tagID` | H5 只读详情 SSR（零外链，void/404 双态） |
| GET/HEAD `/qr/:urlCode?size=` | **服务端 QR 端点（V1.14）**：渲染活码 URL 的 PNG；`internal/qr` 薄封装 skip2/go-qrcode（size 钳 64-1024 缺省 512）；复用 lookupTagID 校验（void→404 与分流一致）；`Cache-Control: public, max-age=86400`（tagId 永不 recycle 支撑长缓存） |
| GET `/api/v1/assets/:tagID` | 资产详情 JSON `{asset, events}`（小程序数据源） |
| POST `/api/v1/auth/login` | 员工登录（`WECHAT_APPID` 未配 = dev 模式 mock openid；配凭证走真 code2session） |

### 员工端（/api/v1，HMAC token 鉴权）

`GET /auth/me` · `GET /auth/my-events` · `POST /assets/:tagID/actions`（快捷操作四动作，强制写 asset_events）

### 管理端（/api/admin，ADMIN_PASSWORD + HMAC；空密码 = open 免鉴权形态）

| 组 | 端点 |
|---|---|
| 台账/码 | GET/POST `/assets` · PUT `/assets/:tagID` · GET `/tags` · GET `/tags/scan-stats`（聚合） · POST `/tags/:tagID/void` · GET `/config`（下发 BASE_URL 单一事实源） |
| 导入 | POST `/import`（**dedup 默认 true**，force 逃生口） · POST `/import/precheck`（只读预检：新建/跳过/疑似三态） |
| 清洗（V1.15 P2） | POST `/cleanup/scan`（预标 delete/void 双 action） · POST `/cleanup/execute`（confirm_count 防误触） · GET `/cleanup/jobs`（审计） · POST `/cleanup/jobs/:jobID/restore`（快照回补） |
| 财务（M4） | GET/PUT `/fin/mappings` · POST `/fin/export`（审计留痕） · GET `/fin/export/jobs` · POST `/fin/import/preview`（含 `_tag_id` 直匹配通道） · GET `/fin/import/batches` · POST `/fin/import/batches/:batchID/confirm` · `.../rollback` |
| 统计 | GET `/stats` |
| SPA | `/admin`（`internal/adminui` embed 构建产物 + SPA fallback） |

### 处理器文件地图（internal/api）

`router.go`（Deps+路由）· `resolve.go`（302 分流+lookupTagID 缓存通道）· `handlers.go`（资产 CRUD/统计/config）· `admin.go`（管理端鉴权+码管理+导入）· `mp.go`（员工端）· `fin.go`（财务域）· `cleanup.go`（清洗三步流）· `dedupe.go`（键归一化 min/max 幂等）· `qr.go`（QR 端点）· `auth.go`/`authn.go`（管理/员工双鉴权）

## 关键依赖与配置

**go.mod 直接依赖（4 个）**：gin v1.12.0 · pgx/v5 v5.13.0（simple protocol 强制，免疫 pooler 42P05）· skip2/go-qrcode pseudo-version `v0.0.0-20200617195104-da1b6568686e`（无 git tag，pseudo-version 必须从 goproxy @latest 取真值）· modernc.org/sqlite v1.60.1（纯 Go 零 CGO）

**环境变量**（`internal/config/config.go`）：

| 变量 | 缺省 | 说明 |
|---|---|---|
| `LISTEN_ADDR` | :8080 | 监听地址（生产 18000） |
| `SQLITE_PATH` | data/mutag.db | SQLite 文件（开发形态） |
| `DATABASE_URL` | 空 | **形态开关**：有值 → PG，空 → SQLite |
| `BASE_URL` | http://localhost:8080 | 活码对外域名（QR 内容/前端码构造单一事实源） |
| `ADMIN_PASSWORD` | 空（免鉴权） | 管理后台密码 |
| `WECHAT_APPID`/`WECHAT_SECRET` | 空 | 空 = 员工登录 dev mock 模式 |
| `TOKEN_SECRET` | dev-…-change-me | 员工 token HMAC 密钥 |
| `WEAPP_JUMP_URL` | 空 | 302 微信内跳转目标（配后启用小程序唤起） |
| `RATE_LIMIT_BURST`/`RATE_LIMIT_PER_SEC` | 30 / 5 | IP 令牌桶（≤0 关闭） |

## 数据模型（迁移 `internal/db/migrations/`，双方言独立版本线）

| 迁移 | 表 | 说明 |
|---|---|---|
| 0001 | `tags` / `assets` / `asset_events` / `scan_logs` | 码（url_code, active/void，**tagId 永不 recycle**）/ 资产 / 履历快照 / 扫码日志 |
| 0002 | `users` | openid + 角色（admin/asset_mgr/staff） |
| 0003 | `fin_mappings` / `export_jobs` / `import_batches` | 财务模板（内置四模板幂等播种）/ 导出审计 / 导入批次（可回滚） |
| 0004 | `cleanup_jobs` | 清洗审计（快照 4 表回补 / PG setval 序列重推） |

文件命名：SQLite `NNNN_*.up/down.sql`；PG 带中缀 `NNNN_*.pg.up/down.sql`（`db.SelectUpMigrationsByDialect` 按方言挑版本线）。repo 层 `?` 占位符双方言一份 SQL（`internal/db/rebind_driver.go` 包装 pgx 做 `?`→`$n` rebind + 引号保护；PG 侧 `RETURNING` 替代 LastInsertId）。

`internal/repo` 文件地图：`repo.go`（Store 聚合+EnsureFinDefaults 播种）· `asset_repo.go` · `tag_repo.go` · `event_repo.go` · `scan_log_repo.go` · `user_repo.go` · `fin_repo.go` · `cleanup_repo.go`（单事务原子清理+restore+快照）

`internal/cache`：`cache.go`（TTL 60s，tagId→资产，变更即失效）· `limiter.go`（IP 令牌桶，Redis 替换点——上线只换此文件实现，Allow 语义不变）

## 测试与质量

**21 个测试函数**（`go test ./...`；SQLite 全量 + `MUTAG_TEST_PG_DSN` 有值时全套 e2e 跑真 PG，清 schema 重迁移）：

- `internal/api`（14）：TestE2EFullChain（管理全链）/ TestE2EEmployeeChain / TestE2EAuthMode（open/鉴权双形态）/ TestE2EFinanceChain / TestE2EGridEdit / TestE2EImportDedupe（六场景）/ TestE2ECleanupChain / TestE2ERateLimit / TestE2EScanStats / TestE2EAppConfig / TestE2EQRCode（七断言）/ TestE2EHomeAndNoRoute / TestResolveTarget / TestScanChannel
- `internal/cache`（3）：令牌桶容量回填 / key 隔离 / 关闭态
- `internal/db`（2）：rebind / 方言迁移挑选
- `internal/repo`（1）：nextTagSeq 解析；`internal/h5`（1）：模板渲染

e2e 为**进程内**集成测试（net/http/httptest 起 router，绕沙箱端口限制）。部署实证：106 容器内 TestE2EQRCode PASS；公网 `https://mutag.qingmulife.com/qr/MT-2026-000001` 200。

## 常见问题 (FAQ)

1. **沙箱构建失败？** TLS MITM 拦 Go proxy：`GOFLAGS="-mod=mod -buildvcs=false" GOPROXY=off GOSUMDB=off` 走模块缓存；GOCACHE/GOMODCACHE 指可写目录（沙箱拒写 ~/Library/Caches，可指 APFS 克隆目录 `cp -Rc`）。
2. **skip2/go-qrcode 依赖拉不动？** 无 git tag；go.sum 双哈希已入库（2026-11-17 与 sum.golang.org 权威记录比对锚定）。alpine 容器缺 git 跑不动 tidy → 用 `docker.1ms.run/golang:1.26`（Debian 自带 git）。
3. **Supabase pooler 报 42P05/3F000？** simple protocol 已强制（OpenPostgres 自动追加）；生产推荐 db.<ref>:5432 直连（118 形态）；106 无 IPv6 出口走 session pooler :5432——两形态并存不强求统一。
4. **物理删 tags 会怎样？** 码号回池（nextTagID=MAX+1 实证）——纪律 1 禁止，一律置 void。
5. **待办（已批准未实施）**：`internal/qr` 升级 withLogo（纠错 Medium→High + 中心白垫 22% + logo 贴图 18% + go:embed 资产）、`/qr/:urlCode?logo=0` 开关、e2e 补 bytes 断言。规划见 `../.zcf/plan/current/qr-logo.md`。**勿在文档/代码中当作已完成。**

## 相关文件清单

- 入口：`cmd/server/main.go` · `cmd/migrate/main.go` · `Dockerfile`
- API：`internal/api/router.go`（先读这个）· `internal/api/qr.go` · `internal/api/dedupe.go` · `internal/api/cleanup.go`
- 存储：`internal/db/db.go` · `internal/db/rebind_driver.go` · `internal/db/migrations/`（16 文件）
- 渲染：`internal/h5/h5.go` + `internal/h5/templates/{_common,detail,notfound}.html`（UI 设计 token 全表）
- QR：`internal/qr/qr.go`（ClampSize/Render；logo 升级点）
- 数据种子：`scripts/xlsx_to_json.py` · `scripts/seed/{main.go,data.json}`（真实数据 74 条源）
- 管理后台 embed：`internal/adminui/adminui.go` + `internal/adminui/dist/`（由 `admin-web/build.sh` 覆盖写入，勿手改）
