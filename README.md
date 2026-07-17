# 硬盘唤醒追踪器

`fn-disk-wakeup-tracker` 是飞牛 fnOS 原生常驻应用，用于低干扰地记录机械硬盘状态变化，并以“疑似来源 + 证据 + 可信度”的形式辅助分析唤醒原因。首版只监控、记录、分析和诊断，不修改系统休眠策略，也不自动处置其他进程。

## 设计重点

- Go Server 以应用专用用户运行，仅监听 `${TRIM_APPDEST}/app.sock`，通过统一网关 `/app/fn-disk-wakeup-tracker` 访问。
- Collector 负责 `/proc`、`/sys` 和白名单硬件查询，可由生命周期脚本以 root 启动；它不暴露网络端口。
- 高频样本只保存在内存，只有状态变化、异常和低频聚合才通过单写入队列批量写 SQLite。
- 机械盘状态探测会先检查能力；SSD/NVMe 返回 `unsupported`，无法无损判断时返回 `unknown`，不会把未知当待机。
- 所有 API 都校验飞牛网关提供的 `X-Trim-Userid`、`X-Trim-Isadmin` 和 `X-Trim-Username`，仅管理员可访问。

## 安装位置与自唤醒风险

应用的数据库和日志位于 fnOS 提供的 `${TRIM_PKGVAR}`。应用未强制设置 `install_type=root`，因为系统分区安装和卸载保留语义需要在目标 fnOS 版本上真机验证。安装时应优先选择 SSD 所在存储空间；如果把应用数据放在被监测的机械盘上，数据库聚合写入和日志仍可能影响该盘休眠。

状态探测默认只使用只读 sysfs/procfs 信息，并将潜在硬件查询关闭，因此无法无损确认时会显示“未知”。完成真机对照验证后，管理员可以显式启用仅针对已扫描 SATA/SAS 机械盘、固定参数的 `hdparm -C` 查询。不同控制器、USB 桥接芯片和固件行为可能不同，必须按 [真机验收清单](docs/fnos-acceptance.md) 完成 30 分钟以上对照验证。

## 权限与数据目录

生命周期以受控 root 上下文启动 Collector，但通过 `runuser` 让统一网关 Server 长期以 `TRIM_USERNAME` 专用包用户运行。应用只监听两个 Unix Socket，不开放 TCP 端口。配置、数据库、日志和临时数据分别使用 fnOS 提供的 `TRIM_PKGETC`、`TRIM_PKGVAR`、`TRIM_PKGTMP` 和 `TRIM_PKGHOME`，不硬编码安装卷。

卸载向导默认保留配置和历史数据；选择删除时只清理经过归属和危险路径校验的本应用目录。fnOS 卸载后是否自动清理这些目录仍是[真机待验证项](docs/architecture.md#卸载数据选择)，当前文档不声称已经恢复验证通过。

## 版本机制

根目录 `VERSION` 是唯一发布版本源。manifest 使用模板占位，Go 的 version/commit/buildTime/platform 使用 `-ldflags` 注入，Vite 使用 `APP_VERSION` 注入 UI，FPK 文件名、输出目录和 SHA256 清单均由构建脚本生成。`./scripts/check-version.sh` 会检查语义版本、CHANGELOG、manifest 模板和代码硬编码。

## 本地开发

要求：Go 1.25+、Node.js 20+、npm，以及发布打包时所需的官方 `fnpack`。

```bash
go test ./...
cd web && npm ci && npm run test && npm run build
./scripts/build.sh x86
./scripts/build.sh arm
./scripts/build.sh all
make release
```

构建产物输出到 `dist/`，源码目录不会写入二进制或前端构建产物。最终 FPK 不包含 Node.js、Python、Docker、`node_modules` 或其他外部运行时。

更完整的架构、数据库和卸载说明见 [docs/architecture.md](docs/architecture.md)，API 见 [docs/api.md](docs/api.md)，构建发布步骤见 [docs/build.md](docs/build.md)。

## 当前限制

- 硬盘电源状态与卸载保留行为尚待 fnOS x86/ARM 真机验证。
- USB 桥接、多路径、RAID、LVM 和 device-mapper 会尽力解析到底层物理盘；无法可靠解析时明确标记不支持。
- 疑似来源来自时间窗口内 I/O 相关性，不代表确定因果关系。
