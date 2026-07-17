#!/bin/bash
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [ -z "$ROOT" ] || [ "$ROOT" = "/" ]; then echo "拒绝清理危险路径" >&2; exit 1; fi
rm -rf -- "$ROOT/build" "$ROOT/dist" "$ROOT/web/dist"
echo "已清理构建产物"

