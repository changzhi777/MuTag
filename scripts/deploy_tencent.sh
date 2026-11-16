#!/bin/bash
# deploy_tencent.sh MuTag 腾讯云一键部署（Supabase 形态 · 单二进制 + systemd）
# 用法（须先加 SSH 别名 mutag 并 export DATABASE_URL）：
#   DATABASE_URL='postgres://...?sslmode=require' bash scripts/deploy_tencent.sh mutag
# 可选：WEB_PORT=18000（远端服务端口，默认 18000）
set -e
cd "$(dirname "$0")/.."   # 仓根

HOST_ALIAS="${1:-mutag}"
WEB_PORT="${WEB_PORT:-18000}"
APP_DIR="/opt/mutag"
BIN_LOCAL="$TMPDIR/mutag-linux-amd64"

[ -n "$DATABASE_URL" ] || { echo "缺 DATABASE_URL（Supabase 连接串）"; exit 1; }
command -v ssh >/dev/null || exit 1

echo "=== 1. 构建 admin SPA + 交叉编译 linux/amd64 ==="
(cd admin-web && pnpm build --silent)
rm -rf backend/internal/adminui/dist
cp -r admin-web/dist backend/internal/adminui/dist
(cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS="-mod=mod -buildvcs=false" \
  go build -ldflags "-s -w" -o "$BIN_LOCAL" ./cmd/server)
ls -lh "$BIN_LOCAL"

echo "=== 2. 远端探测（架构 / 端口占用）==="
ssh "$HOST_ALIAS" "uname -m; ss -tlnp | grep -q ':$WEB_PORT ' && { echo \"端口 $WEB_PORT 已占用，请 WEB_PORT=xxx 重跑\"; exit 9; } || echo PORT_$WEB_PORT\_FREE"

echo "=== 3. 上传二进制 ==="
ssh "$HOST_ALIAS" "mkdir -p $APP_DIR"
scp -q "$BIN_LOCAL" "$HOST_ALIAS:$APP_DIR/mutag"

echo "=== 4. 生成 ADMIN_PASSWORD 并安装 systemd ==="
ADMIN_PW=$(head -c 16 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 16)
ssh "$HOST_ALIAS" "mkdir -p $APP_DIR && cat > $APP_DIR/mutag.env <<ENVEOF
LISTEN_ADDR=:$WEB_PORT
BASE_URL=http://43.129.201.118:$WEB_PORT
DATABASE_URL=$DATABASE_URL
ADMIN_PASSWORD=$ADMIN_PW
ENVEOF
cat > /etc/systemd/system/mutag.service <<UNIT
[Unit]
Description=MuTag 活码服务
After=network-online.target

[Service]
WorkingDirectory=$APP_DIR
EnvironmentFile=$APP_DIR/mutag.env
ExecStart=$APP_DIR/mutag
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT
chmod 600 $APP_DIR/mutag.env
systemctl daemon-reload && systemctl enable --now mutag
sleep 1 && systemctl is-active mutag"

echo "=== 5. 验收 ==="
sleep 1
ssh "$HOST_ALIAS" "curl -s http://127.0.0.1:$WEB_PORT/healthz; echo; curl -s -o /dev/null -w 'admin SPA: %{http_code}\n' http://127.0.0.1:$WEB_PORT/admin/; curl -s -o /dev/null -w '未知码: %{http_code}\n' http://127.0.0.1:$WEB_PORT/t/MT-2026-999999"
curl -s --max-time 8 "http://43.129.201.118:$WEB_PORT/healthz" && echo " ← 公网可达"

echo ""
echo "✅ 部署完成：http://43.129.201.118:$WEB_PORT"
echo "   管理后台 ADMIN_PASSWORD：$ADMIN_PW（已写入远端 $APP_DIR/mutag.env，请自行保存）"
