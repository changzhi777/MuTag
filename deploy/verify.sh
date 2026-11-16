#!/bin/bash
# MuTag M1 一键验收（SQLite 形态，零 Docker 依赖）：
# 重置库 → 编译 → 起服务(:18080) → seed 74 条 → 双链路 curl 六连检
set -e
cd "$(dirname "$0")/../backend"

export GOFLAGS="-mod=mod -buildvcs=false"
export GOCACHE="${GOCACHE:-$TMPDIR/gocache}"

echo "=== 1. 编译 ==="
go build ./...
go build -o "$TMPDIR/mutag-server" ./cmd/server

echo "=== 2. 重置库 + 起服务(:18080) + seed ==="
rm -rf data
LISTEN_ADDR=:18080 "$TMPDIR/mutag-server" &
SERVER_PID=$!
sleep 2
trap "kill $SERVER_PID 2>/dev/null || true" EXIT

go run ./scripts/seed

echo "=== 3. 双链路验收 ==="
set +e
BASE="http://localhost:18080"
TAG="MT-2026-000001"
pass=0; fail=0
check() {  # check "<描述>" "<实际输出>" "<期望子串>"
  if echo "$2" | grep -qF "$3"; then
    echo "PASS: $1"; pass=$((pass+1))
  else
    echo "FAIL: $1 （实际: $2）"; fail=$((fail+1))
  fi
}

echo "--- 3.1 活码 302"
R=$(curl -s -o /dev/null -w "%{http_code} %{redirect_url}" "$BASE/t/$TAG"); echo "$R"
check "302 → /h5/t/$TAG" "$R" "302 $BASE/h5/t/$TAG"

echo "--- 3.2 H5 详情页"
R=$(curl -s "$BASE/h5/t/$TAG" | grep -oE '<h1>[^<]+</h1>' | head -1); echo "$R"
check "H5 渲染资产名" "$R" "<h1>"

echo "--- 3.3 微信 UA 分流（M1 仍落 H5）"
R=$(curl -s -o /dev/null -w "%{http_code} %{redirect_url}" -A "Mozilla/5.0 MicroMessenger/8.0" "$BASE/t/$TAG"); echo "$R"
check "微信 UA 302" "$R" "302 $BASE/h5/t/$TAG"

echo "--- 3.4 JSON API"
R=$(curl -s "$BASE/api/v1/assets/$TAG" | head -c 120); echo "$R"
check "JSON 含 asset 与 events" "$R" '"asset"'

echo "--- 3.5 healthz"
R=$(curl -s "$BASE/healthz"); echo "$R"
check "健康探针" "$R" '"status":"ok"'

echo "--- 3.6 不存在的码"
R=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/t/MT-2026-999999"); echo "$R"
check "未知码 404" "$R" "404"

echo "--- 3.7 末号码 MT-2026-000074"
R=$(curl -s "$BASE/h5/t/MT-2026-000074" | grep -oE '<h1>[^<]+</h1>' | head -1); echo "$R"
check "末号码渲染" "$R" "<h1>"

echo "--- 3.8 管理后台 SPA（应 200 + HTML）"
R=$(curl -s "$BASE/admin/codes" | head -c 60); echo "$R"
check "SPA fallback 出 HTML" "$R" "<!DOCTYPE html>"

echo "--- 3.9 管理端登录（免鉴权形态 open:true）"
R=$(curl -s -X POST "$BASE/api/admin/login" -H 'Content-Type: application/json' -d '{"password":"x"}'); echo "$R"
check "login open 形态" "$R" '"open":true'

echo "--- 3.10 管理端新建资产（应自动配码 MT-2026-000075）"
R=$(curl -s -X POST "$BASE/api/admin/assets" -H 'Content-Type: application/json' \
  -d '{"name":"验收测试资产","unit":"个","keeper":"admin"}'); echo "$R"
check "新建配码 000075" "$R" "MT-2026-000075"
NEW_TAG="MT-2026-000075"

echo "--- 3.11 管理端编辑（写 update 履历；PUT 全量替换，须带全字段——模拟前端编辑表单真实提交）"
R=$(curl -s -X PUT "$BASE/api/admin/assets/$NEW_TAG" -H 'Content-Type: application/json' \
  -d '{"name":"验收测试资产（已改名）","unit":"个","keeper":"admin","status":"in_use"}'); echo "$R"
check "编辑变更生效" "$R" '"changed":1'

echo "--- 3.12 管理端码作废 → 扫码应 200「已作废」"
curl -s -X POST "$BASE/api/admin/tags/$NEW_TAG/void" > /dev/null
R=$(curl -s "$BASE/h5/t/$NEW_TAG" | grep -oE '已作废' | head -1); echo "$R"
check "void 码 200 渲染已作废" "$R" "已作废"

echo "--- 3.13 统计接口"
R=$(curl -s "$BASE/api/admin/stats" | head -c 120); echo "$R"
check "stats 含 by_status" "$R" '"by_status"'

echo "--- 3.14 财务模板内置播种（M4，应≥4 个）"
R=$(curl -s "$BASE/api/admin/fin/mappings" | grep -o '"mapping_id"' | wc -l | tr -d ' '); echo "模板数: $R"
[ "$R" -ge 4 ] && R='"mapping_id"' || R="missing"
check "财务模板播种" "$R" '"mapping_id"'

echo "--- 3.15 财务导出（模板映射 + 审计留痕；seed 74 + 3.10 新建 1 = 75 条全量，含报废）"
R=$(curl -s -X POST "$BASE/api/admin/fin/export" -H 'Content-Type: application/json' \
  -d '{"mapping_id":"generic"}'); echo "$R" | head -c 200; echo ""
check "导出行数与审计" "$R" '"row_count":75'

echo ""
echo "=== 验收汇总：PASS $pass / FAIL $fail ==="
[ "$fail" -eq 0 ] && echo "出口标准「外部扫码看到资产页」达成 ✅"
echo "小程序端：微信开发者工具打开 miniprogram/（勾选不校验合法域名，apiBase 改 http://localhost:18080）"
