#!/bin/bash
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION_FILE="$ROOT/VERSION"
if [ ! -f "$VERSION_FILE" ]; then echo "错误：缺少 VERSION" >&2; exit 1; fi
VERSION="$(tr -d '[:space:]' < "$VERSION_FILE")"
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then echo "错误：VERSION 不是合法语义版本：$VERSION" >&2; exit 1; fi
if ! grep -Fxq "## $VERSION" "$ROOT/CHANGELOG.md"; then echo "错误：CHANGELOG.md 缺少当前版本标题 ## $VERSION" >&2; exit 1; fi
if ! grep -Fxq "version=@VERSION@" "$ROOT/packaging/fnos/manifest"; then echo "错误：manifest 必须由 @VERSION@ 占位并在 staging 注入" >&2; exit 1; fi
if grep -R --line-number --fixed-strings 'Version = "0.' "$ROOT/internal" "$ROOT/cmd" 2>/dev/null; then echo "错误：Go 版本号不得写死" >&2; exit 1; fi
if grep -R --line-number --fixed-strings 'const APP_VERSION' "$ROOT/web/src" 2>/dev/null; then echo "错误：UI 版本号必须由构建注入" >&2; exit 1; fi
echo "版本一致性检查通过：$VERSION"

