#!/bin/bash
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="${1:-all}"
case "$TARGET" in all|x86|arm) ;; *) echo "用法：$0 [all|x86|arm]" >&2; exit 2 ;; esac
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
BUILD_ROOT="$ROOT/build"
OUT_DIR="$ROOT/dist/$VERSION"
COMMIT="$(git -C "$ROOT" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
export GOCACHE="$ROOT/.cache/go-build"
export npm_config_cache="$ROOT/.cache/npm"
mkdir -p "$GOCACHE" "$npm_config_cache"

echo "== 硬盘唤醒追踪器构建 =="
echo "版本: $VERSION"
echo "提交: $COMMIT"
echo "时间: $BUILD_TIME"
if ! git -C "$ROOT" diff --quiet 2>/dev/null || ! git -C "$ROOT" diff --cached --quiet 2>/dev/null; then
  echo "提示：Git 工作区存在未提交修改。"
  if [ "${REQUIRE_CLEAN:-0}" = "1" ]; then echo "错误：REQUIRE_CLEAN=1 时拒绝脏工作区构建" >&2; exit 1; fi
fi

"$ROOT/scripts/check-version.sh" || exit 1
"$ROOT/scripts/check-comments.sh" || exit 1
command -v go >/dev/null 2>&1 || { echo "错误：未找到 Go" >&2; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "错误：未找到 npm（仅构建阶段需要，FPK 运行时不需要）" >&2; exit 1; }

echo "-- 安装锁定的前端依赖"
NPM_INSTALL_ARGS=(install --no-save --package-lock=false --no-audit --no-fund)
if [ "${NPM_OFFLINE:-0}" = "1" ]; then NPM_INSTALL_ARGS+=(--offline); fi
(cd "$ROOT/web" && npm "${NPM_INSTALL_ARGS[@]}") || exit 1
echo "-- 前端类型检查与测试"
(cd "$ROOT/web" && npm run typecheck && npm run test:run) || exit 1
echo "-- 构建前端"
(cd "$ROOT/web" && APP_VERSION="$VERSION" npm run build) || exit 1
echo "-- Go 单元测试与 vet"
UNFORMATTED="$(gofmt -l "$ROOT/cmd" "$ROOT/internal")"
if [ -n "$UNFORMATTED" ]; then echo "错误：以下 Go 文件未格式化：" >&2; echo "$UNFORMATTED" >&2; exit 1; fi
(cd "$ROOT" && go test ./... && go vet ./...) || exit 1

mkdir -p "$BUILD_ROOT/bin" "$BUILD_ROOT/staging" "$OUT_DIR"

# build_arch 为单一目标架构创建二进制和独立 staging，防止跨架构串包。
build_arch() {
  local platform="$1"
  local goarch="$2"
  local bin_dir="$BUILD_ROOT/bin/$platform"
  mkdir -p "$bin_dir"
  local ldflags="-s -w -X github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/version.Version=$VERSION -X github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/version.Commit=$COMMIT -X github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/version.BuildTime=$BUILD_TIME -X github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/version.Platform=$platform"
  echo "-- 构建 Linux $goarch"
  (cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath -ldflags "$ldflags" -o "$bin_dir/fn-disk-wakeup-server" ./cmd/server) || exit 1
  (cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath -ldflags "$ldflags" -o "$bin_dir/fn-disk-wakeup-collector" ./cmd/collector) || exit 1
  "$ROOT/scripts/package.sh" "$platform" "$bin_dir" "$OUT_DIR" || exit 1
}

case "$TARGET" in
  x86) build_arch x86 amd64 ;;
  arm) build_arch arm arm64 ;;
  all) build_arch x86 amd64; build_arch arm arm64 ;;
esac

(cd "$OUT_DIR" && shasum -a 256 ./*.fpk > SHA256SUMS) || exit 1
echo "-- 构建摘要"
ls -lh "$OUT_DIR"
echo "输出目录: $OUT_DIR"
