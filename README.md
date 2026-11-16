# MuTag（青沐动态码）· 企业资产活码管理

每个资产贴一个"码不变、内容随时改"的活码：扫码即看资产全生命周期，管理后台改信息即时生效，财务导出对上用友/金蝶模板。

## 仓库结构

```
backend/       Go 1.26 + Gin 活码服务（SQLite 开发形态 / PG 上线形态）
  cmd/server/    服务入口（启动即自动迁移）
  internal/      domain / repo / api / h5 / db / cache / config / adminui
  scripts/       seed 数据导入 + xlsx 转换
miniprogram/   原生微信小程序（员工端：扫码 + 详情）
admin-web/     管理后台（React 19 + AntD 6：台账/码管理/导入/财务对接/统计）
deploy/        验收脚本 + PG 上线形态 compose
```

## 快速开始（SQLite，零外部依赖）

```bash
cd backend
go run ./cmd/server                    # :8080，启动即自动迁移四表
python3 scripts/xlsx_to_json.py "../固定资产需打标签.xlsx" scripts/seed/data.json
go run ./scripts/seed                  # 64 行 xlsx → 一物一码拆分 74 条资产

curl -i localhost:8080/t/MT-2026-000001   # 302 → H5 详情
```

**管理后台**：浏览器开 `http://localhost:8080/admin`（默认免鉴权；设 `ADMIN_PASSWORD=xxx` 启用密码登录）。重建前端：`bash admin-web/build.sh`。

一键验收：`bash deploy/verify.sh`（15 连检：活码双链路 + 管理端 + M4 财务模板/导出审计，端口 :18080）。

**发版**：`sh scripts/next_version.sh calc patch`（算号）/ `apply patch`（三件套 + commit + tag + push 全自动；工作区必须干净）。当前发布版 V1.0.1（锚定 M1-M4 全量）。

**财务对接**（后台「财务对接」页）：内置用友 U8 固定资产卡片 / 金蝶云星空资产清单 / 管家婆 / 通用台账四模板（列映射可视化编辑，version 递增）；导出按模板映射生成 XLSX（金额两位小数 / 状态中文 / 日期格式均由映射层转换），后端留操作人+过滤条件审计单；导入按模板反向解析 → 差异预览（SN 匹配判定新增/更新，错误行标红原因）→ 人工确认入库 → 支持按批次回滚（新增删除+码置 void，更新按旧值快照恢复）。

**表格编辑**（后台「表格编辑」页）：台账全量网格行内编辑（react-data-grid，改动列高亮，资产码只读）→ 改动集差异预览 → 确认入库（审计/回滚全复用）。

微信小程序：开发者工具打开 `miniprogram/`（测试号，勾选"不校验合法域名"，apiBase 对齐服务端口）。

## 核心链路

| 入口 | 行为 |
|---|---|
| `GET /t/{urlCode}` | 活码 302 分流器（缓存 60s → 库 → UA 分流 → 写 scan_logs，HEAD 兼容） |
| `GET /h5/t/{tagID}` | H5 只读详情页（SSR，零外链，void 200 / 不存在 404 双态） |
| `GET /api/v1/assets/{tagID}` | JSON：资产 + 履历（小程序数据源） |
| `GET /healthz` | 存活探针 |

一物一码：`MT-2026-000001` 起顺序编号，tagId 永不 recycle，作废置 void 归档。

## 环境变量

`LISTEN_ADDR`(:8080) · `SQLITE_PATH`(data/mutag.db) · `BASE_URL`

## 开发约束（重要）

- **离线构建**：本机沙箱代理 TLS 拦截 Go proxy，依赖走模块缓存（gin v1.12.0 / modernc sqlite v1.60.1），`GOFLAGS="-mod=mod -buildvcs=false" GOPROXY=off GOSUMDB=off`；在线环境正常 `go mod tidy` 即可
- **迁移器**：自研轻量迁移器（`internal/db`，schema_migrations 版本表 + embed SQL，幂等）
- **存储形态**：SQLite（modernc 纯 Go）+ 内存 TTL 缓存为开发/验收形态；PG17+Redis7 为上线形态（`deploy/docker-compose.yml` 保留，切换点见 deploy/README.md）
- **seed 幂等**：库中已有资产时拒绝重复导入；两步导入（xlsx → JSON → 入库）
