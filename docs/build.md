# 构建与发布

## 工具

- macOS Apple Silicon 开发机
- Go 1.25 或更高版本
- Node.js 20 与 npm（只在构建阶段使用）
- 飞牛官方当前版 `fnpack`；构建会打印实际版本，不锁死唯一工具版本

## 命令

```bash
./scripts/check-comments.sh
./scripts/check-version.sh
./scripts/build.sh x86
./scripts/build.sh arm
./scripts/build.sh all
make release
```

脚本依次执行注释/版本检查、按现有 lock 安装前端依赖且禁止改写 lock、前端类型检查和测试、前端生产构建、Go 测试和 vet、`CGO_ENABLED=0` 双架构编译、独立 staging 校验、`fnpack build` 和 SHA256 生成。

预期输出目录是 `dist/<VERSION>/`：

```text
fn-disk-wakeup-tracker-x86-<VERSION>.fpk
fn-disk-wakeup-tracker-arm-<VERSION>.fpk
SHA256SUMS
```

缺少 `fnpack` 时脚本会在完成源码、测试、二进制和 staging 检查后明确失败，不会创建或声称存在 FPK。发布前必须在干净工作区以 `REQUIRE_CLEAN=1 ./scripts/build.sh all` 重跑，并完成 [fnOS 真机验收](fnos-acceptance.md)。

依赖已存在于 npm 缓存且构建机网络受限时，可使用 `NPM_OFFLINE=1`；离线缓存不完整会明确失败，不会绕过依赖安装。
