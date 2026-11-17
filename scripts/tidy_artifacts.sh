#!/usr/bin/env bash
# 操作产物自动归档：仓库根目录散落的截图(png) / 浏览器快照(yml) → 约定目录。
# 用法：
#   scripts/tidy_artifacts.sh                 # 归档（默认）
#   scripts/tidy_artifacts.sh --dry-run       # 仅预览，不移动
#   scripts/tidy_artifacts.sh --quiet         # 静默（pre-commit 用）
#   scripts/tidy_artifacts.sh --install-hook  # 挂载 pre-commit（core.hooksPath .githooks）
# 护栏：git 已跟踪文件一律不动；目标同名追加后缀，永不覆盖。
set -euo pipefail
cd "$(dirname "$0")/.."

PNG_DIR="deploy/tencent/screenshots"   # 运维截图（gitignore）
YML_DIR=".playwright-mcp/snapshots"    # 浏览器/Playwright 快照（gitignore）

quiet=0; dry=0
for a in "$@"; do
  case "$a" in
    --dry-run) dry=1 ;;
    --quiet) quiet=1 ;;
    --install-hook)
      mkdir -p .githooks
      cat > .githooks/pre-commit <<'HOOK'
#!/usr/bin/env bash
# 提交前自动归档根目录散落的截图/快照（scripts/tidy_artifacts.sh）
"$(git rev-parse --show-toplevel)/scripts/tidy_artifacts.sh" --quiet
HOOK
      chmod +x .githooks/pre-commit
      git config core.hooksPath .githooks
      echo "已挂载 pre-commit：git config core.hooksPath .githooks（提交即自动整理）"
      exit 0 ;;
    *) echo "未知参数：$a（支持 --dry-run / --quiet / --install-hook）"; exit 1 ;;
  esac
done

shopt -s nullglob
moved=0
tidy() { # $1=glob模式 $2=目标目录 $3=类型标签
  local pattern="$1" dest="$2" tag="$3" f base name ext target i
  for f in $pattern; do
    # 护栏：git 已跟踪的文件一律不动（正式配置/文档永不被归档）
    if git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
      continue
    fi
    mkdir -p "$dest"
    name="$(basename "$f")"; base="${name%.*}"; ext="${name##*.}"
    target="$dest/$name"
    if [ -e "$target" ]; then
      for i in 1 2 3 4 5 6 7 8 9; do
        target="$dest/$base-$i.$ext"; [ -e "$target" ] || break
      done
      [ -e "$target" ] && target="$dest/$base-$(date +%Y%m%d%H%M%S).$ext"
    fi
    if [ "$dry" = 1 ]; then
      echo "[预览] $f → $target"
    else
      mv "$f" "$target"
    fi
    moved=$((moved + 1))
  done
}

tidy '*.png' "$PNG_DIR" "截图"
tidy '*.yml' "$YML_DIR" "快照"

if [ "$quiet" = 1 ]; then
  [ "$moved" -gt 0 ] && echo "tidy: 已归档 $moved 个产物文件"
else
  if [ "$dry" = 1 ]; then
    echo "共 $moved 个待归档（--dry-run 未实际移动）"
  else
    echo "归档完成：$moved 个文件 → $PNG_DIR / $YML_DIR"
  fi
fi
