#!/bin/bash
set -u
if [ "$#" -ne 3 ]; then echo "用法：$0 <stage> <platform> <version>" >&2; exit 2; fi
STAGE="$1"; PLATFORM="$2"; VERSION="$3"

# file_mode 兼容 macOS 与 Linux 的 stat 参数，返回不含前导零的 POSIX 权限。
file_mode() {
  local path="$1"
  if stat -f '%Lp' "$path" >/dev/null 2>&1; then stat -f '%Lp' "$path"; else stat -c '%a' "$path"; fi
}

# require_mode 校验单个 staging 路径的权限，防止目录不可遍历或静态文件带执行位。
require_mode() {
  local path="$1"
  local expected="$2"
  local actual
  actual="$(file_mode "$path")"
  if [ "$actual" != "$expected" ]; then
    echo "错误：权限不正确 $path，期望 $expected，实际 $actual" >&2
    exit 1
  fi
}
for file in manifest config/privilege config/resource app/ui/config ICON.PNG ICON_256.PNG cmd/main wizard/install wizard/upgrade wizard/uninstall wizard/config app/bin/fn-disk-wakeup-server app/bin/fn-disk-wakeup-collector app/web/index.html; do
  if [ ! -e "$STAGE/$file" ]; then echo "错误：打包缺少 $file" >&2; exit 1; fi
done
for file in "$STAGE/config/privilege" "$STAGE/config/resource" "$STAGE/app/ui/config" "$STAGE/wizard/install" "$STAGE/wizard/upgrade" "$STAGE/wizard/uninstall" "$STAGE/wizard/config"; do
  node -e 'JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"))' "$file" || exit 1
done
grep -Fxq "appname=fn-disk-wakeup-tracker" "$STAGE/manifest" || exit 1
grep -Fxq "version=$VERSION" "$STAGE/manifest" || exit 1
grep -Fxq "platform=$PLATFORM" "$STAGE/manifest" || exit 1
if find "$STAGE" \( -name node_modules -o -name .git -o -name '.env' -o -name '.env.*' -o -name 'test.db' -o -name Dockerfile -o -name '*.log' \) -print -quit | grep -q .; then echo "错误：staging 包含禁止路径或文件" >&2; exit 1; fi
if grep -R -I -E '(BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY|Authorization:[[:space:]]*Bearer|password[[:space:]]*=)' "$STAGE" >/dev/null 2>&1; then echo "错误：staging 文本包含疑似密钥或凭据" >&2; exit 1; fi
for script in "$STAGE"/cmd/*; do if [ ! -x "$script" ]; then echo "错误：脚本不可执行 $script" >&2; exit 1; fi; done
while IFS= read -r directory; do require_mode "$directory" 755; done < <(find "$STAGE" -type d -print)
while IFS= read -r file; do
  case "$file" in
    "$STAGE"/cmd/*|"$STAGE"/app/bin/*) require_mode "$file" 755 ;;
    *) require_mode "$file" 644 ;;
  esac
done < <(find "$STAGE" -type f -print)
node -e 'const fs=require("fs");for(const [f,w,h] of [[process.argv[1],64,64],[process.argv[2],256,256]]){const b=fs.readFileSync(f);if(b.toString("hex",1,4)!="504e47"||b.readUInt32BE(16)!==w||b.readUInt32BE(20)!==h)throw new Error(`${f} 尺寸无效`)}' "$STAGE/ICON.PNG" "$STAGE/ICON_256.PNG" || exit 1
echo "staging 校验通过：platform=$PLATFORM version=$VERSION"
