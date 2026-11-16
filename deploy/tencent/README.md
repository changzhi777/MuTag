# MuTag 腾讯云生产部署（43.129.201.118 · Lighthouse 香港 lhins-42zqxraf）

> 首次上线：2026-11-17 · 形态：Supabase PG17 + 单二进制 + systemd + nginx TLS
> 实例名：MuTag（轻量控制台可改）· Debian · 2核4G · /dev/vda1 69G

## 拓扑

```
微信/浏览器 ──HTTPS──> nginx :443 (certbot 托管 Let's Encrypt, 自动续期)
                        └─ proxy_pass ──> mutag :18000 (systemd, Supabase pooler 6543)
活码 DNS: mutag.qingmulife.com A → 43.129.201.118 (TTL 600)
管理后台: https://mutag.qingmulife.com/admin/
```

## 目录（统一保存裁决）

```
/opt/mutag/
├── mutag          # 二进制（go build ./cmd/server，31M 含 embed admin SPA）
├── mutag.env      # 环境变量（600 权限）：LISTEN_ADDR/BASE_URL/DATABASE_URL/ADMIN_PASSWORD
└── src/           # 仓库克隆（git pull 更新源码后重编译）
/etc/systemd/system/mutag.service
/etc/nginx/sites-available/mutag.conf  （sites-enabled 软链）
```

## 本文件即配置快照（与线上一致）

- `mutag.service` / `mutag.nginx.conf`：与本目录同名文件一一对应，重装时直接覆盖回位
- 证书续期：certbot timer 自动（`certbot renew`）

## 更新发布流程

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
