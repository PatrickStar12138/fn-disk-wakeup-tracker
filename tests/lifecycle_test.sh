#!/bin/bash
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# Unix Socket 在 macOS 上路径长度较短，因此测试目录固定放在短路径 /tmp 下。
TMP="$(mktemp -d "/tmp/fn-dw-life.XXXXXX")"

# cleanup 只清理测试记录过的进程和临时目录，避免失败用例残留后台服务。
cleanup() {
  local file
  local pid
  for file in "$TMP"/*.observed "$TMP"/runuser.pid; do
    if [ ! -f "$file" ]; then continue; fi
    pid="$(sed -n '1p' "$file" | tr -d '[:space:]')"
    case "$pid" in ''|*[!0-9]*) continue ;; esac
    kill -TERM "$pid" 2>/dev/null || true
  done
  rm -rf -- "$TMP"
}
trap cleanup EXIT

# fail_test 输出明确的回归测试失败原因并终止测试。
fail_test() {
  echo "$1" >&2
  if [ -f "${TRIM_TEMP_LOGFILE:-}" ]; then tail -n 20 "$TRIM_TEMP_LOGFILE" >&2; fi
  if [ -f "${TRIM_PKGVAR:-}/logs/server.log" ]; then tail -n 20 "${TRIM_PKGVAR}/logs/server.log" >&2; fi
  if [ -f "${TRIM_PKGVAR:-}/logs/collector.log" ]; then tail -n 20 "${TRIM_PKGVAR}/logs/collector.log" >&2; fi
  exit 1
}

# watch_process_identity 等待生命周期脚本写 PID，再构造仅供 macOS 测试使用的 /proc 身份链接。
watch_process_identity() {
  local pid_file="$1"
  local expected="$2"
  local observed="$3"
  local count=0
  local pid
  while [ ! -f "$pid_file" ] && [ "$count" -lt 200 ]; do sleep 0.05; count=$((count + 1)); done
  [ -f "$pid_file" ] || return 1
  pid="$(sed -n '1p' "$pid_file" | tr -d '[:space:]')"
  case "$pid" in ''|*[!0-9]*) return 1 ;; esac
  printf '%s\n' "$pid" > "$observed"
  mkdir -p "$FN_DISK_PROC_ROOT/$pid"
  # 与 Linux /proc/<pid>/exe 一致，测试身份链接指向规范化后的真实可执行文件路径。
  ln -s "$(realpath "$expected")" "$FN_DISK_PROC_ROOT/$pid/exe"
}

# wait_process_exit 有界等待指定测试进程退出，防止仅凭 PID 文件删除误判停止成功。
wait_process_exit() {
  local pid="$1"
  local count=0
  while kill -0 "$pid" 2>/dev/null && [ "$count" -lt 100 ]; do sleep 0.05; count=$((count + 1)); done
  ! kill -0 "$pid" 2>/dev/null
}
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
export FN_DISK_PROC_ROOT="$TMP/proc"
export FN_DISK_RUNUSER_BIN="$ROOT/tests/fixtures/fake-runuser"
export FN_DISK_TEST_RUNUSER_PID_FILE="$TMP/runuser.pid"
export FN_DISK_STARTUP_TIMEOUT_SECONDS=2
export GOCACHE="$ROOT/.cache/go-build"
export GOMODCACHE="$ROOT/.cache/go-mod"
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

export TRIM_PKGVAR="$BASE/var"
mkdir -p "$TRIM_APPDEST/bin" "$FN_DISK_PROC_ROOT"
# 前序数据保留用例写入的是故意损坏的占位数据库；进程测试必须从独立空数据库开始。
rm -f -- "$TRIM_PKGVAR/db/tracker.db"
(cd "$ROOT" && go build -o "$TRIM_APPDEST/bin/fn-disk-wakeup-server" ./cmd/server) || fail_test "无法构建生命周期测试 Server"
(cd "$ROOT" && go build -o "$TRIM_APPDEST/bin/fn-disk-wakeup-collector" ./cmd/collector) || fail_test "无法构建生命周期测试 Collector"
SERVER_BIN="$TRIM_APPDEST/bin/fn-disk-wakeup-server"
COLLECTOR_BIN="$TRIM_APPDEST/bin/fn-disk-wakeup-collector"
SERVER_PID_FILE="$TRIM_PKGVAR/run/server.pid"
COLLECTOR_PID_FILE="$TRIM_PKGVAR/run/collector.pid"

# 正常启动场景同时验证真实 PID、runuser 包装 PID、重复启动和真实进程停止。
watch_process_identity "$SERVER_PID_FILE" "$SERVER_BIN" "$TMP/server-normal.observed" & server_watch=$!
watch_process_identity "$COLLECTOR_PID_FILE" "$COLLECTOR_BIN" "$TMP/collector-normal.observed" & collector_watch=$!
bash "$ROOT/packaging/fnos/cmd/main" start || fail_test "正常生命周期 start 失败"
wait "$server_watch" && wait "$collector_watch" || fail_test "未能记录正常启动进程身份"
server_pid="$(sed -n '1p' "$SERVER_PID_FILE")"
collector_pid="$(sed -n '1p' "$COLLECTOR_PID_FILE")"
runuser_pid="$(sed -n '1p' "$FN_DISK_TEST_RUNUSER_PID_FILE")"
[ "$server_pid" != "$runuser_pid" ] || fail_test "Server PID 错误指向 runuser 包装进程"
bash "$ROOT/packaging/fnos/cmd/main" status || fail_test "真实 Server 与 Collector 状态校验失败"
bash "$ROOT/packaging/fnos/cmd/main" start || fail_test "重复 start 应保持成功"
[ "$(sed -n '1p' "$SERVER_PID_FILE")" = "$server_pid" ] || fail_test "重复 start 启动了第二个 Server"
bash "$ROOT/packaging/fnos/cmd/main" stop || fail_test "正常 stop 失败"
wait_process_exit "$server_pid" || fail_test "stop 未终止真实 Server"
wait_process_exit "$collector_pid" || fail_test "stop 未终止真实 Collector"
[ ! -e "$TRIM_APPDEST/app.sock" ] && [ ! -e "$TRIM_PKGVAR/run/collector.sock" ] || fail_test "stop 未清理本应用 Socket"
bash "$ROOT/packaging/fnos/cmd/main" stop || fail_test "重复 stop 应保持幂等"

# 网关 Socket 超时场景使用不会监听 Socket 的 Collector 替代 Server，验证回滚不会留下孤儿进程。
cp "$SERVER_BIN" "$TMP/server.good"
cp "$COLLECTOR_BIN" "$SERVER_BIN"
rm -f -- "$TMP/server-timeout.observed" "$FN_DISK_TEST_RUNUSER_PID_FILE"
watch_process_identity "$SERVER_PID_FILE" "$SERVER_BIN" "$TMP/server-timeout.observed" & timeout_watch=$!
if FN_DISK_STARTUP_TIMEOUT_SECONDS=2 bash "$ROOT/packaging/fnos/cmd/main" start 2>/dev/null; then fail_test "Server 网关 Socket 超时未返回失败"; fi
wait "$timeout_watch"
timeout_pid="$(sed -n '1p' "$TMP/server-timeout.observed")"
wait_process_exit "$timeout_pid" || fail_test "Server 就绪超时后留下孤儿进程"
grep -q 'Server 网关 Socket 创建超时' "$TRIM_TEMP_LOGFILE" || fail_test "网关 Socket 超时错误未写入用户日志"
grep -q 'Server 网关 Socket 创建超时' "$TRIM_PKGVAR/logs/lifecycle.log" || fail_test "网关 Socket 超时错误未写入应用日志"
cp "$TMP/server.good" "$SERVER_BIN"

# Collector 启动失败场景验证已就绪 Server 会被真实终止，而不是只删除 PID 文件。
cp "$COLLECTOR_BIN" "$TMP/collector.good"
cp /usr/bin/false "$COLLECTOR_BIN"
rm -f -- "$TMP/server-collector-fail.observed" "$FN_DISK_TEST_RUNUSER_PID_FILE"
watch_process_identity "$SERVER_PID_FILE" "$SERVER_BIN" "$TMP/server-collector-fail.observed" & server_fail_watch=$!
if FN_DISK_STARTUP_TIMEOUT_SECONDS=2 bash "$ROOT/packaging/fnos/cmd/main" start 2>/dev/null; then fail_test "Collector 启动失败未返回错误"; fi
wait "$server_fail_watch"
failed_server_pid="$(sed -n '1p' "$TMP/server-collector-fail.observed")"
wait_process_exit "$failed_server_pid" || fail_test "Collector 启动失败后 Server 未被停止"
grep -Eq 'Collector (进程启动失败|PID 文件无效|进程身份不匹配)' "$TRIM_TEMP_LOGFILE" || fail_test "Collector 启动失败未写入具体错误"
cp "$TMP/collector.good" "$COLLECTOR_BIN"

# 损坏或陈旧 PID 文件场景验证身份不匹配时只清理 PID 文件，不误杀其他进程。
/bin/sleep 30 & unrelated_pid=$!
printf '%s\n' "$unrelated_pid" > "$SERVER_PID_FILE"
mkdir -p "$FN_DISK_PROC_ROOT/$unrelated_pid"
ln -s /bin/sleep "$FN_DISK_PROC_ROOT/$unrelated_pid/exe"
bash "$ROOT/packaging/fnos/cmd/main" stop
kill -0 "$unrelated_pid" 2>/dev/null || fail_test "损坏 PID 文件误杀了无关进程"
[ ! -e "$SERVER_PID_FILE" ] || fail_test "损坏 PID 文件未清理"
kill -TERM "$unrelated_pid" 2>/dev/null || true

echo "生命周期静态、数据保留、真实 PID、启动回滚与幂等停止测试通过"
