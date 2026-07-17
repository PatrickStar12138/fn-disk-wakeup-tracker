#!/bin/bash
set -u

if [ "$#" -ne 1 ]; then echo "用法：$0 <staging-dir>" >&2; exit 2; fi
STAGE="$1"
if [ -z "$STAGE" ] || [ ! -d "$STAGE" ]; then echo "错误：staging 目录不存在" >&2; exit 1; fi
if [ ! -d "$STAGE/cmd" ] || [ ! -d "$STAGE/app/bin" ]; then echo "错误：staging 缺少 cmd 或 app/bin 目录" >&2; exit 1; fi

# 先统一去除复制来源携带的执行位，再只恢复目录、生命周期脚本与 Go 二进制的必要权限。
find "$STAGE" -type d -exec chmod 0755 {} + || exit 1
find "$STAGE" -type f -exec chmod 0644 {} + || exit 1
chmod 0755 "$STAGE"/cmd/* "$STAGE/app/bin/"* || exit 1
