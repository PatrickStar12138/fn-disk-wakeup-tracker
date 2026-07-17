#!/bin/bash
set -u

if [ "$#" -ne 3 ]; then echo "用法：$0 <x86|arm> <binary-dir> <output-dir>" >&2; exit 2; fi
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PLATFORM="$1"
BIN_DIR="$2"
OUT_DIR="$3"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
STAGE="$ROOT/build/staging/$PLATFORM"
PACKAGE_NAME="fn-disk-wakeup-tracker-$PLATFORM-$VERSION.fpk"

case "$PLATFORM" in x86|arm) ;; *) echo "错误：平台无效" >&2; exit 1 ;; esac
if [ ! -x "$BIN_DIR/fn-disk-wakeup-server" ] || [ ! -x "$BIN_DIR/fn-disk-wakeup-collector" ]; then echo "错误：缺少目标架构二进制" >&2; exit 1; fi
SERVER_FILE="$(file "$BIN_DIR/fn-disk-wakeup-server")"
case "$PLATFORM" in
  x86) echo "$SERVER_FILE" | grep -Eq 'x86-64|x86_64' || { echo "错误：x86 staging 二进制架构不匹配：$SERVER_FILE" >&2; exit 1; } ;;
  arm) echo "$SERVER_FILE" | grep -Eq 'aarch64|ARM64|arm64' || { echo "错误：arm staging 二进制架构不匹配：$SERVER_FILE" >&2; exit 1; } ;;
esac
echo "二进制架构: $SERVER_FILE"
rm -rf -- "$STAGE"
mkdir -p "$STAGE/app/bin" "$STAGE/app/web" "$OUT_DIR"
cp -R "$ROOT/packaging/fnos/." "$STAGE/" || exit 1
cp "$BIN_DIR/fn-disk-wakeup-server" "$STAGE/app/bin/"
cp "$BIN_DIR/fn-disk-wakeup-collector" "$STAGE/app/bin/"
cp -R "$ROOT/web/dist/." "$STAGE/app/web/"
sed -e "s/@VERSION@/$VERSION/g" -e "s/@PLATFORM@/$PLATFORM/g" "$ROOT/packaging/fnos/manifest" > "$STAGE/manifest"
chmod 0755 "$STAGE"/cmd/* "$STAGE/app/bin/"*

"$ROOT/scripts/test-package.sh" "$STAGE" "$PLATFORM" "$VERSION" || exit 1
FNPACK="${FNPACK_BIN:-}"
if [ -z "$FNPACK" ]; then FNPACK="$(command -v fnpack 2>/dev/null || true)"; fi
if [ -z "$FNPACK" ] || [ ! -x "$FNPACK" ]; then
  echo "错误：未找到官方 fnpack。请从 https://developer.fnnas.com/docs/cli/fnpack/ 下载适用于 macOS Apple Silicon 的当前版本并加入 PATH。" >&2
  echo "已完成二进制、前端和 staging 校验，但不会声称生成 FPK。" >&2
  exit 1
fi
FNPACK_VERSION="$("$FNPACK" --help 2>&1 | sed -n '/^Version /p' | head -n 1)"
echo "fnpack: ${FNPACK_VERSION:-版本信息不可用}"
rm -f -- "$ROOT/fn-disk-wakeup-tracker.fpk" "$STAGE/fn-disk-wakeup-tracker.fpk"
(cd "$ROOT" && "$FNPACK" build --directory "$STAGE") || exit 1
GENERATED=""
for candidate in "$ROOT/fn-disk-wakeup-tracker.fpk" "$STAGE/fn-disk-wakeup-tracker.fpk" "$ROOT/build/staging/fn-disk-wakeup-tracker.fpk"; do
  if [ -f "$candidate" ]; then GENERATED="$candidate"; break; fi
done
if [ -z "$GENERATED" ]; then echo "错误：fnpack 返回成功但未找到生成的 FPK" >&2; exit 1; fi
mv -f -- "$GENERATED" "$OUT_DIR/$PACKAGE_NAME"
echo "已生成: $OUT_DIR/$PACKAGE_NAME"
