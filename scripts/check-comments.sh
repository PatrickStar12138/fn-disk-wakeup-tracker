#!/bin/bash
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
command -v node >/dev/null 2>&1 || { echo "错误：注释检查需要构建环境中的 Node.js" >&2; exit 1; }
node "$ROOT/scripts/check-comments.mjs" "$ROOT"

