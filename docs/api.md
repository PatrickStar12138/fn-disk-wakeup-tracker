# API v1

所有接口位于 `/app/fn-disk-wakeup-tracker/api/v1`，只接受统一网关转发的管理员上下文。错误响应为 `{"error":"面向用户的信息","status":HTTP状态码}`，不会返回 SQL、系统路径或堆栈。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/version` | version、commit、buildTime、platform |
| GET | `/overview` | 状态数量、今日唤醒、服务和数据库状态 |
| GET | `/disks` | 最近持久化的物理盘状态，不触发硬件扫描 |
| GET | `/events` | 分页事件，支持 range、diskId、type、confidence、source |
| GET | `/events/export.csv` | 按筛选条件每批 200 条流式导出 |
| GET | `/settings` | 当前已校验设置 |
| PUT | `/settings` | 最大 64 KiB，请求限流并校验全部范围 |
| POST | `/refresh` | 请求下一安全采样周期刷新，不立即访问硬件 |
| GET | `/diagnostics` | 页面脱敏诊断预览 |
| GET | `/diagnostics.txt` | 下载脱敏文本报告 |
| GET | `/diagnostics.json` | 下载脱敏 JSON 报告 |

事件 `range` 仅允许 `24h`、`7d`、`30d`；`page` 从 1 开始，`pageSize` 最大 200。所有修改接口重新验证管理员身份、限制方法和调用频率。

