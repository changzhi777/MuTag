#!/bin/bash
# next_version.sh 版本规程：V1.0.x 第 3 位自动递增（inkforge 基座同款）
#   sh scripts/next_version.sh calc patch   → 算下一版本号（只算不发）
#   sh scripts/next_version.sh apply patch  → 一条命令全自动：三件套 + commit + tag + push
# 纪律：apply 要求工作区干净（防半挂状态发版）；版本锚定已验证状态。
set -e
cd "$(dirname "$0")/.."

MODE="${1:-}"
BUMP="${2:-patch}"

# calc 取最新 V* tag，第 3 位 +1（V1.0 前缀固定场景按第 3 位数字序；升位时手动 tag）
calc() {
  local latest
  latest=$(git tag -l 'V*' | sort -t. -k3 -n | tail -1)
  if [ -z "$latest" ]; then
    echo "V1.0.1"
  else
    echo "$latest" | awk -F. -v OFS=. '{$3=$3+1; print}'
  fi
}

case "$MODE" in
  calc)
    calc
    ;;
  apply)
    if [ -n "$(git status --porcelain)" ]; then
      echo "拒绝发版：工作区有未提交变更（版本必须锚定已验证状态）" >&2
      exit 1
    fi
    V=$(calc)
    SHORT=$(git rev-parse --short HEAD)
    NOW=$(date +"%Y-%m-%d %H:%M %Z")

    echo "V$V" > VERSION
    printf '{"version":"%s","released_at":"%s","git_sha":"%s"}\n' \
      "$V" "$(date -u +%Y-%m-%dT%H:%M:%S+00:00)" "$SHORT" > version.json

    # CHANGELOG 新条目插顶部
    if [ -f CHANGELOG.md ]; then
      printf '## %s (%s)\n\n- 发版：见 git log %s..HEAD 与 CLAUDE.md changelog\n\n' \
        "$V" "$NOW" "$SHORT" | cat - CHANGELOG.md > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
    else
      printf '# MuTag 版本变更记录\n\n## %s (%s)\n\n- 首版\n\n' "$V" "$NOW" > CHANGELOG.md
    fi

    git add VERSION version.json CHANGELOG.md
    git commit -m "chore: release $V"
    git tag "$V"
    git push origin main --tags
    echo "✅ 已发布 $V (git_sha=$SHORT)"
    ;;
  *)
    echo "用法: sh scripts/next_version.sh {calc|apply} [patch]" >&2
    exit 1
    ;;
esac
