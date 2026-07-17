#!/bin/bash
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/fn-disk-wakeup-permissions.XXXXXX")"
trap 'rm -rf -- "$TMP"' EXIT
STAGE="$TMP/staging"
cp -R "$ROOT/packaging/fnos/." "$STAGE"
mkdir -p "$STAGE/app/bin" "$STAGE/app/web"
printf '#!/bin/bash\nexit 0\n' > "$STAGE/app/bin/fn-disk-wakeup-server"
printf '#!/bin/bash\nexit 0\n' > "$STAGE/app/bin/fn-disk-wakeup-collector"
printf '<!doctype html><title>test</title>\n' > "$STAGE/app/web/index.html"
printf 'body {}\n' > "$STAGE/app/web/app.css"
printf 'console.log("test")\n' > "$STAGE/app/web/app.js"
sed -e 's/@VERSION@/0.1.0/g' -e 's/@PLATFORM@/x86/g' "$ROOT/packaging/fnos/manifest" > "$STAGE/manifest"

# 先模拟来源目录和静态文件都被错误设为 0775，再验证权限规范化与严格检查均能通过。
chmod -R 0775 "$STAGE"
bash "$ROOT/scripts/set-package-permissions.sh" "$STAGE"
bash "$ROOT/scripts/test-package.sh" "$STAGE" x86 0.1.0

# 静态文件一旦误带执行位，严格检查必须失败，随后恢复权限供目录反例继续使用。
chmod 0755 "$STAGE/app/web/app.css"
if bash "$ROOT/scripts/test-package.sh" "$STAGE" x86 0.1.0 >/dev/null 2>&1; then
  echo "静态文件执行权限未被打包检查拒绝" >&2
  exit 1
fi
chmod 0644 "$STAGE/app/web/app.css"

# config 和 wizard 目录缺少执行权限时，严格检查必须失败。
chmod 0644 "$STAGE/wizard"
if bash "$ROOT/scripts/test-package.sh" "$STAGE" x86 0.1.0 >/dev/null 2>&1; then
  echo "wizard 目录缺少执行权限未被打包检查拒绝" >&2
  exit 1
fi
chmod 0755 "$STAGE/wizard"
echo "打包目录与文件权限回归测试通过"
