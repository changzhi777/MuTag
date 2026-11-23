# MuTag 腾讯云部署（双机拓扑 · 2026-11-17 生产入口迁至广州 106）

> 首次上线：2026-11-17（香港 118 · systemd）→ **生产入口迁移：2026-11-17 广州 106（Docker，用户裁定）**
> 当前活码主入口：**https://mutag.qingmulife.cn → 广州 106.53.168.73**；香港 118 同库待机（systemd 二进制形态）

## 当前生产拓扑（广州 106 · VM-0-12-debian · 4核8G Lighthouse lhins-c285olfp）

```
DNS: mutag.qingmulife.cn A → 106.53.168.73（TTL 600，DNSPod 权威 subway/amelia 双 NS 已验证）
浏览器/微信 ──HTTPS──> nginx :443 (certbot 托管, /etc/nginx/sites-enabled/mutag.conf)
                      └─ proxy_pass ──> Docker mutag :127.0.0.1:18000→8080
存储：Supabase PG17（session pooler aws-0-ap-southeast-2.pooler.supabase.com:5432 IPv4 + simple protocol）
管理后台: https://mutag.qingmulife.cn/admin/（ADMIN_PASSWORD 见 /opt/mutag/mutag.env）
```

- 目录：`/opt/mutag/`（mutag.env 600 · src = github.com/changzhi777/MuTag clone · data/）
- 镜像构建：`cd /opt/mutag/src && docker build -f backend/Dockerfile -t mutag:latest backend`
- 容器重建（⚠️ `docker restart` 不重读 --env-file，改 env 必须 rm + run）：
  `docker rm -f mutag && docker run -d --name mutag --restart always -p 127.0.0.1:18000:8080 -v /opt/mutag/data:/app/data -e SQLITE_PATH=/app/data/mutag.db --env-file /opt/mutag/mutag.env mutag:latest`
- nginx conf 归档：[mutag-gz106.nginx.conf](./mutag-gz106.nginx.conf)（与线上一致）

## 香港待机（118 · lhins-42zqxraf · systemd 二进制形态 · 2026-11-24 已同步最新）

**实态（2026-11-24）**：mutag 服务 **active**（此前 stop+disable 的裁定已被后续操作恢复——V1.17 qr-logo 公网验证即经此机）；工作区已 `git reset --hard origin/main` 对齐 `75d4306`，go build（`/usr/local/go/bin/go`，PATH 未含需 export）+ restart 完成，公网三验过（healthz / APK-UA 302 / QR logo 10926 字节）。**分叉遗留**：V1.14 时期在 118 本地直接 commit 过三个改动（根路径落地页/NoRoute 兜底等，未进 GitHub），内容已被主线完整包含，reset 前已留 `backup-118-fork` 分支于 118 本地备查。⚠️ **SSH 22 Connection refused**（2026-11-24 实测，非 Clash 掐——sshd 未监听或控制台防火墙变更），运维通道当前**仅 OrcaTerm**（控制台实例卡片「登录」，Playwright MCP 可自主操作）。
BASE_URL 亦为 .cn；nginx `.cn` conf 见 [mutag-cn.nginx.conf](./mutag-cn.nginx.conf)（DNS 已不指向 118）。
更新发布（118）：OrcaTerm → `cd /opt/mutag/src && git fetch origin && git reset --hard origin/main && export PATH=$PATH:/usr/local/go/bin && cd backend && GOFLAGS="-mod=mod -buildvcs=false" go build -ldflags "-s -w" -o /opt/mutag/mutag ./cmd/server && systemctl restart mutag`

## 目录（香港 118 · 统一保存裁决）

```
/opt/mutag/
├── mutag          # 二进制（go build ./cmd/server，31M 含 embed admin SPA）
├── mutag.env      # 环境变量（600 权限）：LISTEN_ADDR/BASE_URL/DATABASE_URL/ADMIN_PASSWORD
│                  #   + WECHAT_APPID/WECHAT_SECRET（沐禾小程序 wx8c37d7ac5b7d0a83，真实 openid 登录）
├── backups/       # 每日 pg_dump（保留 14 份）
├── backup.sh      # 备份脚本（cron 03:00）
└── src/           # 仓库克隆（git pull 更新源码后重编译）
/etc/systemd/system/mutag.service
/etc/nginx/sites-available/mutag.conf  （sites-enabled 软链）
```

## 本文件即配置快照（与线上一致）

- `mutag.service` / `mutag.nginx.conf`：与本目录同名文件一一对应，重装时直接覆盖回位
- 证书续期：certbot timer 自动（`certbot renew`）

## 更新发布流程（香港 118）

```bash
ssh mutag   # 或 OrcaTerm（SSH 被 Clash TUN 掐时的通道）
cd /opt/mutag/src && git pull
cd backend && export PATH=$PATH:/usr/local/go/bin
GOFLAGS="-mod=mod -buildvcs=false" go build -ldflags "-s -w" -o /opt/mutag/mutag ./cmd/server
systemctl restart mutag
```

## 数据备份（Supabase 免费层无自动备份，自建每日 cron）

- 脚本：`/opt/mutag/backup.sh`（root 700）——PG17 pg_dump **直连** Supabase（db.<ref>.supabase.co:5432，
  非 pooler：pg_dump 需要会话语义）→ `/opt/mutag/backups/mutag-YYYYMMDD-HHMM.dump`（custom 格式），保留 14 份
- cron：`0 3 * * * /opt/mutag/backup.sh >> /opt/mutag/backups/backup.log 2>&1`
- ⚠️ **pg_dump 版本必须 ≥ 服务器版本**（Debian 自带 15.x 无法 dump PG17，需 PGDG 仓库装 `postgresql-client-17`：
  `/usr/lib/postgresql/17/bin/pg_dump`——2026-11-17 首次备份踩坑实录）
- 恢复：`pg_restore -d <DSN> --clean <dump 文件>`

## 首次部署实录（2026-11-17）与排障记录

1. **盘满**：根分区 100%（69G）——`/var/lib/docker` 40G，107 镜像 90% 孤儿。
   `docker image prune -af` 回收 30.15GB → 53%。⚠️ 教训：prune 前先 `docker ps -a` 确认容器
   （同机沐禾健身 muhe-fit-api/PG/Redis + rustdesk hbbs/hbbr 运行中，误删不可逆）
2. **SSH 断连**：`Connection closed` 系本机 Clash TUN 掐 22 端口（服务器 sshd 正常）。
   运维通道：OrcaTerm 网页终端（控制台实例卡片「登录」）；根治：Clash 加 `43.129.201.118 DIRECT`
3. **Go 安装**：apt 版过旧，手动 tarball（`go.dev/dl/go1.26.2.linux-amd64.tar.gz` → /usr/local/go）
4. **TLS**：机上 nginx 已占 80/443（qingmulife 站反代），Caddy 不可用 → nginx server block +
   `certbot --nginx -d mutag.qingmulife.com` 自动签发与续期
5. **防火墙**：轻量控制台预设规则已含 80/443；自定义 18000 需手动加（本仓部署前已加）
6. **DNS**：DNSPod qingmulife.com 加 A 记录 `mutag → 43.129.201.118`（TTL 600）
7. **Supabase**：region ap-southeast-2（Sydney）；连接串必须 `sslmode=require`；
   ⚠️ **应用连接务必走直连（db.<ref>.supabase.co:5432），勿用 transaction pooler（6543）**：
   池化连接 search_path 为空 → `ensure schema_migrations: no schema has been selected (SQLSTATE 3F000)`
   启动 crash 循环（2026-11-17 实录，counter 35+）；直连用户名 `postgres`（无 .ref 后缀）；
   备份 pg_dump 亦走直连（且 pg_dump 版本必须 ≥ 服务器版本）；simple protocol 仍保留（免疫其余 pooler 场景）
8. **登录失效**：ADMIN_PASSWORD 生成链路（base64|tr 管道）曾引入不可见字符——重设为可读强密码；
   排障三板斧：`cat mutag.env` → `cat /proc/<MainPID>/environ`（进程真实值）→ 本机 curl login

## 广州 106 运维附录：服务端活码 QR 端点（2026-11-17 上线）

- **端点**：`GET/HEAD /qr/:urlCode?size=512`——服务端渲染活码 URL（`BASE_URL/t/{urlCode}`）PNG 码图；公开 + rateLimit 同桶 + active 码 Cache-Control 86400；void/不存在 → 404（与分流语义一致）
- **实现**：`internal/qr` 薄封装 skip2/go-qrcode（MIT，`v0.0.0-20200617195104-da1b6568686e`，尺寸 clamp 64~1024）；e2e `TestE2EQRCode` 七断言
- **106 部署链特殊处理**：`skip2` 无 git tag → 手写 go.mod 版本易错（pseudo-version 必须从 `goproxy.cn/.../@latest` 取真值）；alpine 容器缺 git 使 tidy 回源卡死 → **用 `docker.1ms.run/golang:1.26`（Debian 自带 git）跑 `go mod tidy`** 生成 go.sum；sed 替换 go.mod 版本注意 `[a-z0-9]*` 不匹配连字符（贪婪 `.*` 才安全）
- **go.sum 已补交（2026-11-17）**：skip2 双哈希入库，与 sum.golang.org 权威记录逐字比对锚定；本机补交方法（沙箱 TLS MITM 拦 Go proxy 时适用）：curl 拉 zip/mod → `GOPROXY=file://` 本地代理 `go mod download -json` 算官方哈希 → `go mod download -json` 自动写入 go.sum（⚠️ -mod=mod 会自动写，勿手动重复追加）；GOMODCACHE/GOCACHE 指向 APFS 克隆目录（`cp -Rc` 零拷贝绕沙箱拒写）

## 广州 106 运维附录：Portainer 可视化容器管理（2026-11-17 上线）

- **入口**：https://ser.qingmulife.cn/docker/（中文界面，admin 账号，密码见 106 `/opt/portainer/admin-credentials.txt`）
- **镜像**：`6053537/portainer-ce:2.45.2`（eysp 汉化版，对应上游 CE 2.45.2 LTS；勿用 latest——停在旧版）；经 `docker.1ms.run` 通道拉取
- **容器**：`portainer`，`-p 127.0.0.1:19000:9000`（仅回环）+ docker.sock + `/opt/portainer/data:/data` + `--base-url /docker`
- **admin 初始化**：`--admin-password-file /adminpw`（挂 `/opt/portainer/adminpw.txt` 单行密码，600）——⚠️ 实测 2.45.2 的 setup_token API（`X-Setup-Token` 头）无法生效（403 missing），`--admin-password-file` 为可靠通道；setup 窗口 5 分钟超时后全局 307 `timeout.html`，需 `docker restart portainer` 重开
- **nginx**（`qm-ser.conf`，备份 `qm-ser.bak.portainer`）：`location = /docker` 301 补斜杠 + `location /docker/` → `proxy_pass http://127.0.0.1:19000/`（**尾斜杠剥前缀**——官方 --base-url 语义）+ WebSocket 头（Upgrade/Connection upgrade）+ 3600s 读写超时；锚点插在 `location /docs/` 之前（sed 标记行 + r 命令注入，无转义风险）
- **验证**：公网登录 → 环境向导 → local（Standalone 29.8.0）→ 容器列表 13/13 运行中（中文 UI 全量生效）

## 广州 106 上线实录（2026-11-17 · 生产入口迁移）

1. **DNSPod 坑：单条「添加记录」UI 不推权威**——控制台显示「开启」但权威 NS（subway/amelia）返回
   NXDOMAIN，等 10 分钟无效；**批量操作通道（txt 文件导入 / 批量修改）秒推权威**。复用通道：
   批量操作 → 修改记录 → 指定域名 + 主机记录筛选 → 勾选 → 填新记录值 → 批量修改（操作确认弹窗）
2. **IPv6-only 直连**：106 无 IPv6 出口，`db.<ref>.supabase.co:5432` 解析为 IPv6 → `network is
   unreachable` crash 循环；106 正解 = **session pooler :5432（IPv4）+ simple protocol**（5432 会话
   粘滞无 42P05 风险；6543 transaction pooler 才有）。118 能直连因其解析到 IPv4——两机连接形态
   不必强求一致
3. **sed 子串坑**：`s|BASE_URL=.*|...|` 会误命中 `DATABASE_URL=`（DA**BASE_URL=**）行——多行 env
   改写务必行首锚定 `^KEY=`
4. **admin assets 404 假象**：`curl -sI`（HEAD）打 `/admin/assets/*.js` 404——路由只注册了
   `r.GET`，HEAD 无路由；`curl -s`（GET）实为 200。浏览器白屏实为旧镜像（dist/assets 入库前
   构建，embed 缺 assets）+ 当日直连 DSN crash 循环叠加；用当前 main 重建镜像即愈：
   `docker build -f backend/Dockerfile -t mutag:latest backend`
5. **certbot**：`certbot --nginx -d mutag.qingmulife.cn -n` 自动改写 conf（443 + 80→301），
   服务器已有账户无需 -m；验证 `curl -s https://mutag.qingmulife.cn/healthz`

## 香港 118 二次排障实录（2026-11-17 · 502 复活 + disabled 修复）

- **症状**：.com 全路由 502（nginx 活着，上游 127.0.0.1:18000 无响应）；.cn（106）同库正常 → 定位纯 118 服务问题
- **死因**：`mutag.service` 当日 10:53:01 被正常 stop（journalctl `Stopping mutag.service...服务已退出`，非 crash），
  且 unit 为 **`disabled`** —— 无开机自启，停一次就躺到人工介入
- **处置**：`systemctl start + enable`（补 Created symlink 自启）→ `/opt/mutag/src` git pull →
  `go build -o mutag.new` **原子替换**（构建失败不影响在跑服务）→ restart → 公网全路由复验通过
- **运维通道**：SSH 22 被本机 Clash TUN 掐断（Clash DIRECT 规则疑被冲）→ **OrcaTerm 网页终端全程自主执行**
  （控制台实例卡片「登录」→ 免密连接 TAT/root）；落地页/NoRoute 版本随之上线
