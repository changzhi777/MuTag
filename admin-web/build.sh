#!/bin/bash
# 管理后台构建：防回归闸门 → pnpm build → 产物覆盖 backend/internal/adminui/dist → go build
set -e
cd "$(dirname "$0")"

# 闸门：antd 6 × React 19 下静态 message 静默不渲染，提示必须走 App.useApp()
# （规划：.claude/plan/db-import-clean-dedupe.md 任务 1.3，风格对齐 inkforge 密钥闸门）
# 用 /usr/bin/grep 绝对路径：本机 grep 可能是 ugrep（\b 语义与 exit code 不同）
# 双形态：单行 import 兜子串匹配 / 多行 import 的独立 message 行
GATE=$(/usr/bin/grep -rnE 'import \{[^}]*message[^}]*\} from "antd"' src 2>/dev/null || true)
GATE2=$(/usr/bin/grep -rnE '^message[,:]?$' src --include='*.tsx' 2>/dev/null || true)
if [ -n "$GATE" ] || [ -n "$GATE2" ]; then
  echo "❌ 闸门拦截：检出静态 message 导入（antd 6 × React 19 下静默不渲染）" >&2
  echo "   修复：组件体内 const { message } = App.useApp();（main.tsx 已有 AntdApp 包裹）" >&2
  [ -n "$GATE" ] && echo "$GATE" >&2
  [ -n "$GATE2" ] && echo "$GATE2" >&2
  exit 1
fi

pnpm install
pnpm build

TARGET="../backend/internal/adminui/dist"
rm -rf "$TARGET"
cp -r dist "$TARGET"
echo "已拷贝 dist → backend/internal/adminui/dist"
cd ../backend
GOFLAGS="-mod=mod -buildvcs=false" go build ./cmd/server
echo "后端编译完成（管理后台已内嵌）"
