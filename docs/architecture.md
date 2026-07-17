# 架构与安全说明

## 官方规范依据

2026-07-17 开发前已完整阅读飞牛开放平台当前文档中的“快速开始”全部 4 页、应用框架、Manifest、环境变量、应用权限、应用资源、应用入口、统一网关、用户向导、图标、fnpack、appcenter-cli、Native 应用案例和 2026-07-05 最新更新日志。

实现采用官方包根目录 `app/`、`cmd/`、`config/`、`wizard/`、`manifest` 和两张包图标。统一网关入口注册 `/app/fn-disk-wakeup-tracker`，转发到 target 根目录的 `app.sock`。未声明固定服务端口，也未使用 Docker。官方文档确认 `install_type=root` 是合法值，但没有给出本应用所需的真实介质 I/O 和卸载保留结论，所以首版不强制系统分区安装。

参考：

- https://developer.fnnas.com/docs/quick-started/prerequisites/
- https://developer.fnnas.com/docs/core-concepts/framework/
- https://developer.fnnas.com/docs/core-concepts/manifest/
- https://developer.fnnas.com/docs/core-concepts/environment-variables/
- https://developer.fnnas.com/docs/core-concepts/privilege/
- https://developer.fnnas.com/docs/core-concepts/resource/
- https://developer.fnnas.com/docs/core-concepts/app-entry/
- https://developer.fnnas.com/docs/core-concepts/gateway-registration/
- https://developer.fnnas.com/docs/core-concepts/wizard/
- https://developer.fnnas.com/docs/core-concepts/icon/
- https://developer.fnnas.com/docs/cli/fnpack/
- https://developer.fnnas.com/docs/cli/appcentercli/
- https://developer.fnnas.com/docs/examples/native/
- https://developer.fnnas.com/docs/update-log/20260705/

## 进程和数据流

```text
fnOS 统一网关
  └─ ${TRIM_APPDEST}/app.sock (0660)
       └─ Server（应用专用用户）
            ├─ Vue 静态资源
            ├─ /api/v1 管理员 API
            ├─ SQLite 单连接写入者
            └─ ${TRIM_PKGVAR}/run/collector.sock (0600)
                 └─ Collector（root，仅本地）
                      ├─ /sys/block
                      ├─ /proc/diskstats
                      ├─ 有界 /proc/<pid>/io 扫描
                      └─ 固定模板 hdparm -C（默认关闭）
```

`config/privilege` 使用 root 是为了启动需要读取硬件信息的 Collector。生命周期脚本随后用 `runuser` 把对用户开放的 Server 降权到 `TRIM_USERNAME`。Collector 只连接权限受限的内部 Unix Socket，不监听 TCP，不接收 Web 参数，也没有任意命令接口。

所有网关 API 要求 `X-Trim-Userid` 和 `X-Trim-Username` 非空且 `X-Trim-Isadmin=true`。Socket 权限阻止浏览器绕过统一网关直接构造这些 Header。修改设置和刷新接口有 2 秒用户级限流、明确方法和请求体上限。

## 避免应用自身唤醒硬盘

- 采样间隔默认 15 秒且后端限制为 5–300 秒。
- `hdparm -C` 默认关闭；管理员完成控制器对照测试后才可显式启用。
- 仅扫描系统已发现、匹配 `sd[a-z]+`/`hd[a-z]+` 的旋转设备；命令和参数固定，2 秒超时。
- SSD/NVMe 标记 `unsupported`，能力不足标记 `unknown`。
- 高频 I/O 计数和进程样本只留在内存；磁盘表仅在首次发现、元数据/能力变化、拔插或状态变化时写入。
- 事件和证据在单个 SQLite 事务中写入；统计、保留清理和数据库上限清理为低频任务。
- 进程扫描最多 256 个 PID、每个采样周期一次，不遍历全部 PID 的全部文件描述符。
- Server/Collector 自身 I/O 标记为 `selfActivity`，保留证据但不作为默认疑似来源。
- 生命周期日志仅记录启动/错误，5 MB 时在下次启动轮换；首版保留最多 5 代。
- 不读取被监测盘中的普通文件，不执行完整 SMART 查询，不修改休眠或 hdparm 设置。

## 数据库

SQLite 使用 `modernc.org/sqlite` 纯 Go 驱动，`CGO_ENABLED=0` 可交叉编译 amd64/arm64。数据库使用 WAL、`synchronous=NORMAL`、外键、2 秒 busy timeout 和单连接写入序列化。

| 表 | 用途 |
|---|---|
| `schema_migrations` | 可重复迁移版本 |
| `disks` | 物理盘身份、脱敏信息和最近持久状态 |
| `disk_capabilities` | 状态探测能力和唤醒风险评估 |
| `disk_state_events` | 状态、离线和恢复事件主表 |
| `wake_events` | 唤醒事件的疑似进程/应用/容器摘要 |
| `attribution_evidence` | PID、I/O 增量、时间窗口、可信度和自活动标记 |
| `daily_statistics` | 每日低频聚合 |
| `settings` | 经后端校验的设置快照 |
| `diagnostic_runs` | 有界诊断执行记录 |
| `operation_logs` | 管理员修改、刷新和导出操作日志 |

事件查询强制分页（最多 200 条/页），CSV 每次读取 200 条并流式写出。默认保留 30 天、数据库上限 200 MB，大批删除每批 500 条。

## 卸载数据选择

`wizard/uninstall` 默认 `wizard_uninstall_data=keep`。保留时停止两个进程、关闭 Server/SQLite，并写入 schema、应用版本和保留时间标记，不覆盖旧配置。删除时只接受字面值 `delete`，逐一验证 `TRIM_PKGETC`、`TRIM_PKGVAR`、`TRIM_PKGTMP`、`TRIM_PKGHOME` 非空、不是危险系统路径且末级目录属于当前 `TRIM_APPNAME`，然后才删除其内容。

fnOS 卸载完成后是否继续保留 `etc/var/home` 需要按目标系统版本真机验证。当前实现不把数据复制到未经官方确认的隐藏目录；若系统会自动清理这些目录，应先取得官方支持的持久化机制再发布，不能把当前行为描述为已验证。

