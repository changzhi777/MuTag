[根目录](../CLAUDE.md) > **deploy**

# deploy · MuTag 验收与部署 — 模块文档

> 更新：2026-11-18（init-architect V1.16 首建）｜ bash 验收 + docker-compose 备选形态 + 腾讯云 IaC
> 根上下文：[../CLAUDE.md](../CLAUDE.md) · 部署实录权威：[tencent/README.md](./tencent/README.md)

---

## 变更记录 (Changelog)

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2026-11-16 | — | V1.1 verify.sh 七连检 → V1.5 扩至 15 连检（财务模板播种+导出审计）→ V1.8 deploy/README 三档双轨重写（Supabase 推荐）→ V1.9 Supabase 实证标注 |
| 2026-11-17 | — | V1.10 腾讯云 118 上线（IaC 归档 tencent/：service + nginx conf + 部署实录/排障）→ V1.11 生产首次更新（pooler 3F000 → 切直连）→ V1.12 .cn 双轨（mutag-cn.nginx.conf）→ V1.13 入口定案方案 A → V1.14 广州 106 上线（mutag-gz106.nginx.conf；skip2/go-qrcode 部署坑：pseudo-version 真值 + alpine 缺 git 用 docker.1ms.run/golang:1.26 跑 tidy）+ **Portainer 汉化版附录** |
| 2026-11-18 | V1.0 | 🆕 本模块 CLAUDE.md 首建（init-architect V1.16）：验收/三档形态/双活拓扑/IaC 全景入档 |

---

## 模块职责

三件事：① `verify.sh` 本地一键验收（SQLite 零依赖 15 连检）；② 存储形态三档说明与切换点文档（README.md）；③ 腾讯云生产 IaC 归档与部署/排障实录（tencent/）。生产部署脚本本体在 `../scripts/deploy_tencent.sh`。

## 入口与使用

| 文件 | 说明 |
|---|---|
| `verify.sh` | 一键验收：重置库 → 编译 → 起服务 :18080 → seed 74 条 → **15 连检**（活码 302 / H5 / 微信 UA 分流 / JSON API / healthz / 未知码 404 / 末号码 / SPA 200 / 登录 / 新建自动配码 / 编辑履历 / 码作废 / 统计 / 财务模板播种≥4 / 财务导出审计）。沙箱内端口绑定被禁 → 会话外 `!` 前缀跑 |
| `docker-compose.yml` | 自建 PG17 + Redis7 备选形态（Supabase 为主时不用） |
| `README.md` | 形态三档表 + Supabase 接入步骤（Session pooler :5432 + sslmode=require + simple protocol 要点 + cmd/migrate 先迁移） |

## 双活生产拓扑（2026-11-17 定案）

| 机器 | 域名 | 角色 | 运行形态 | DB 连接 |
|---|---|---|---|---|
| **广州 106**（106.53.168.73） | `mutag.qingmulife.cn` | **活码主入口**（DNS 权威已切） | Docker + nginx TLS（certbot），IaC `tencent/mutag-gz106.nginx.conf` | **session pooler :5432 + simple protocol**（106 无 IPv6 出口连不上 db. 直连） |
| **香港 118**（43.129.201.118） | `mutag.qingmulife.com` | .com 入口继续跑 | systemd + nginx，IaC `tencent/mutag.service` + `mutag.nginx.conf` | **db.<ref>:5432 直连**（pooler 3F000 后全切直连） |

- 数据库同为 Supabase PG17（简单协议保险两机均保留）；两机连接形态**不要求一致**
- 91zm（43.129.205.22，ink.91zm.com.cn）与本产品无关（那是 inkforge 旧域，只读）
- Portainer 汉化版挂 106：`ser.qingmulife.cn/docker/`（镜像 `6053537/portainer-ce:2.45.2`，`--admin-password-file` 通道，nginx 尾斜杠剥前缀 + WebSocket 头——详见 tencent/README.md 运维附录）
- 统一目录裁决：`/opt/mutag/{src,mutag,env}`；管理后台密码见远端 `/opt/mutag/mutag.env`
- 服务端 QR 端点已在 106 部署实证（e2e 七断言 PASS + 公网 `/qr/MT-2026-000001` 200）

## tencent/ IaC 与实录

| 文件 | 内容 |
|---|---|
| `tencent/README.md` | **部署知识库（权威）**：118 上线实录 + 满盘救援 + .cn 双轨 + 106 上线实录（DNSPod 批量通道才推权威 / certbot / skip2 部署坑 / go.sum 离线清偿法）+ 登录排障三板斧 + Portainer 运维附录 |
| `tencent/mutag.service` | systemd 单元（118 形态） |
| `tencent/mutag.nginx.conf` / `mutag-cn.nginx.conf` / `mutag-gz106.nginx.conf` | 三入口 nginx（.com / .cn 301 / 106 主入口） |
| `tencent/screenshots/`（约 200 张） | 全程操作截图归档（gitignore 未跟踪部分由 `../scripts/tidy_artifacts.sh` 自动收拢） |

## 关键依赖与配置

- `../scripts/deploy_tencent.sh`：一键部署（需 SSH 别名 + `DATABASE_URL`；构建 SPA → 交叉编译 linux/amd64 → 远端探测 → scp → 原子替换）
- `../scripts/next_version.sh`：发布版本规程 V1.0.x（calc/apply patch，三件套+tag+push，脏工作区拒绝）
- `../scripts/tidy_artifacts.sh` + `../.githooks/pre-commit`（core.hooksPath 已挂）：提交自动归档根目录散落截图 png → `tencent/screenshots/`、Playwright 快照 yml → `.playwright-mcp/snapshots/`；护栏 = git 已跟踪文件一律不动、同名追加后缀永不覆盖
- 运维通道：SSH 22 被本机 Clash TUN 掐断 → **OrcaTerm 网页终端**（Playwright MCP 有腾讯云登录态可自主操作，免密 TAT/root）；OrcaTerm 编辑器发送对双引号做 `\"` 转义破坏 bash 语法（单引号/零引号命令可行）

## 数据模型

无自有表；compose 形态落 PG17（迁移由 backend `cmd/migrate` 负责）。

## 测试与质量

验收即测试：verify.sh 15 连检 = M1 出口标准 + M3/M4 关键链路的部署后冒烟；线上验收另靠 backend e2e（106 容器内跑过 TestE2EQRCode）+ 公网 curl 断言（healthz / H5 / scan-stats 鉴权拦截 / QR 200）。

## 常见问题 (FAQ)

1. **verify.sh 卡在端口绑定？** 沙箱禁端口绑定，会话外 `!` 前缀跑。
2. **DNSPod 改记录不生效？** 单条「添加记录」UI 不推权威（NXDOMAIN 等 10 分钟）——走批量操作通道（txt 导入/批量修改）秒推。
3. **106 连不上 Supabase？** 无 IPv6 出口连不上 db. 直连，必须 session pooler :5432 + simple protocol；118 反之走直连。
4. **skip2/go-qrcode 部署拉不动？** 无 git tag：pseudo-version 从 goproxy `@latest` 取真值；alpine 缺 git 用 `docker.1ms.run/golang:1.26` 跑 tidy；go.sum 已入库（权威记录比对锚定）。
5. **118 SSH 通道（2026-11-24 修复）**：sshd 听 **22222**（22 跨境被 GFW RST 的有意替代，云防火墙仅放 22222）——`ssh mutag` 已通（本机别名加了 `Port 22222`）；go 需 `export PATH=$PATH:/usr/local/go/bin`。Clash 掐 22 的历史症状与 GFW RST 叠加，勿再误判。qr-logo 已 V1.17 双机落地。

## 相关文件清单

- `verify.sh` · `README.md` · `docker-compose.yml`
- `tencent/README.md`（先读）· `tencent/mutag.service` · `tencent/*.nginx.conf`（3 份）· `tencent/screenshots/`
- 配套脚本：`../scripts/deploy_tencent.sh` · `../scripts/next_version.sh` · `../scripts/tidy_artifacts.sh` · `../.githooks/pre-commit`
