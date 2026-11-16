#!/bin/bash
# 管理后台构建：pnpm build → 产物覆盖 backend/internal/adminui/dist → go build
set -e
cd "$(dirname "$0")"
pnpm install
pnpm build

TARGET="../backend/internal/adminui/dist"
rm -rf "$TARGET"
cp -r dist "$TARGET"
echo "已拷贝 dist → backend/internal/adminui/dist"
cd ../backend
GOFLAGS="-mod=mod -buildvcs=false" go build ./cmd/server
echo "后端编译完成（管理后台已内嵌）"
