#!/bin/bash
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/fn-disk-wakeup-lifecycle.XXXXXX")"
trap 'rm -rf -- "$TMP"' EXIT
BASE="$TMP/fn-disk-wakeup-tracker"
export TRIM_APPNAME="fn-disk-wakeup-tracker"
export TRIM_APPVER="0.1.0"
export TRIM_OLD_APPVER="0.1.0"
export TRIM_APPDEST="$BASE/target"
export TRIM_PKGETC="$BASE/etc"
export TRIM_PKGVAR="$BASE/var"
export TRIM_PKGTMP="$BASE/tmp"
export TRIM_PKGHOME="$BASE/home"
export TRIM_TEMP_LOGFILE="$BASE/error.log"
export TRIM_TEMP_UPGRADE_FOLDER="$BASE/upgrade"
export TRIM_USERNAME="$(id -un)"
export TRIM_GROUPNAME="$(id -gn)"
mkdir -p "$TRIM_APPDEST" "$TRIM_TEMP_UPGRADE_FOLDER"

bash "$ROOT/packaging/fnos/cmd/install_init"
bash "$ROOT/packaging/fnos/cmd/install_callback"
bash "$ROOT/packaging/fnos/cmd/install_callback"
test -f "$TRIM_PKGVAR/data-format"
printf '{}' > "$TRIM_PKGETC/settings.json"
printf 'history' > "$TRIM_PKGVAR/db/tracker.db"
bash "$ROOT/packaging/fnos/cmd/upgrade_init"
test -f "$TRIM_TEMP_UPGRADE_FOLDER/fn-disk-wakeup-tracker-0.1.0/tracker.db"
export wizard_uninstall_data=keep
bash "$ROOT/packaging/fnos/cmd/uninstall_init"
bash "$ROOT/packaging/fnos/cmd/uninstall_callback"
test -f "$TRIM_PKGVAR/db/tracker.db"
export wizard_uninstall_data=delete
bash "$ROOT/packaging/fnos/cmd/uninstall_callback"
test ! -e "$TRIM_PKGVAR/db/tracker.db"

export wizard_uninstall_data=invalid
if bash "$ROOT/packaging/fnos/cmd/uninstall_callback" 2>/dev/null; then echo "无效向导值保护失败" >&2; exit 1; fi

export TRIM_PKGVAR=""
if bash "$ROOT/packaging/fnos/cmd/uninstall_callback" 2>/dev/null; then echo "空路径保护失败" >&2; exit 1; fi
export TRIM_PKGVAR="$TMP/not-owned"
mkdir -p "$TRIM_PKGVAR"
export wizard_uninstall_data=delete
if bash "$ROOT/packaging/fnos/cmd/uninstall_callback" 2>/dev/null; then echo "错误归属路径保护失败" >&2; exit 1; fi
grep -q '^#!/bin/bash' "$ROOT/packaging/fnos/cmd/main"
grep -q 'exit 3' "$ROOT/packaging/fnos/cmd/main"
if grep -q 'pkill -f' "$ROOT/packaging/fnos/cmd/main"; then echo "检测到禁止的模糊 pkill" >&2; exit 1; fi
echo "生命周期静态与数据保留测试通过（进程启停需 Linux/fnOS 验收）"
