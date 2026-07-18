export const API_BASE = '/app/fn-disk-wakeup-tracker/api/v1'

/** Page 是控制台允许切换的七个固定页面标识。 */
export type Page = 'overview' | 'disks' | 'events' | 'sources' | 'diagnostics' | 'settings' | 'about'
/** DiskState 是前后端共享的磁盘状态，unknown 绝不等同于 standby。 */
export type DiskState = 'active' | 'idle' | 'standby' | 'unknown' | 'unsupported'
/** Confidence 是后端归因使用的三级可信度。 */
export type Confidence = '高' | '中' | '低' | ''
/** EventRange 是后端支持的有限事件时间范围。 */
export type EventRange = '24h' | '7d' | '30d'

/** NavigationItem 描述侧栏中固定页面的标签和本地图标。 */
export interface NavigationItem {
  /** id 是七个固定页面之一。 */
  id: Page
  /** label 是导航可访问中文名称。 */
  label: string
  /** icon 是本地 SVG 图标标识。 */
  icon: string
}
/** Disk 描述硬盘列表中的物理盘身份、能力和最近状态。 */
export interface Disk {
  /** id 是数据库中稳定的硬盘标识。 */
  id: string
  /** device 是后端扫描确认的块设备名称，不由前端拼接命令。 */
  device: string
  /** model 是设备报告的型号。 */
  model: string
  /** maskedSerial 是后端已经脱敏的序列号。 */
  maskedSerial: string
  /** capacityBytes 是设备总容量，单位为字节。 */
  capacityBytes: number
  /** busType 是 SATA、USB、NVMe 等总线摘要。 */
  busType: string
  /** rotational 表示设备是否为旋转介质。 */
  rotational: boolean
  /** state 是当前统一状态，unknown 不等同于 standby。 */
  state: DiskState
  /** previousState 是最近一次已确认状态。 */
  previousState: DiskState
  /** lastStateChange 是最近状态变化的 RFC3339 时间。 */
  lastStateChange: string
  /** todayWakeups 是本地日历日内已记录的唤醒次数。 */
  todayWakeups: number
  /** lastActivity 是可选的最近活动时间。 */
  lastActivity?: string
  /** detectionMethod 是后端实际采用的能力或检测方式。 */
  detectionMethod: string
  /** capabilitySupported 表示当前设备是否支持可靠探测。 */
  capabilitySupported: boolean
  /** present 表示设备在最近一次扫描中是否仍然存在。 */
  present: boolean
}
/** WakeEvent 描述带证据摘要的状态或服务事件。 */
export interface WakeEvent {
  /** id 是事件数据库主键。 */
  id: number
  /** diskId 是关联硬盘标识，服务事件可以为空。 */
  diskId: string
  /** device 是事件关联的设备名称。 */
  device: string
  /** type 是后端固定事件枚举。 */
  type: string
  /** fromState 是事件发生前状态。 */
  fromState: DiskState
  /** toState 是事件发生后状态。 */
  toState: DiskState
  /** startedAt 是事件开始时间。 */
  startedAt: string
  /** endedAt 是可选的事件结束时间。 */
  endedAt?: string
  /** durationMs 是单调间隔换算的持续时间，单位为毫秒。 */
  durationMs: number
  /** readDelta 是证据窗口内读取增量，单位为字节。 */
  readDelta: number
  /** writeDelta 是证据窗口内写入增量，单位为字节。 */
  writeDelta: number
  /** suspectProcess 是可选疑似进程名称，不表示确定因果。 */
  suspectProcess: string
  /** suspectFnosApp 是可选疑似 fnOS 应用名称。 */
  suspectFnosApp: string
  /** suspectDockerContainer 是可选疑似容器名称。 */
  suspectDockerContainer: string
  /** reason 是后端保留的时间窗口与 I/O 判断依据。 */
  reason: string
  /** confidence 是证据可信度分级。 */
  confidence: Confidence
  /** note 是事件的有限补充说明。 */
  note: string
}
/** EventPage 是后端有上限的事件分页响应。 */
export interface EventPage {
  /** items 是当前页事件，不代表全部历史记录。 */
  items: WakeEvent[]
  /** page 是当前页码，从 1 开始。 */
  page: number
  /** pageSize 是后端实际采用的单页上限。 */
  pageSize: number
  /** total 是当前筛选条件下的总记录数。 */
  total: number
}
/** EventFilters 是事件列表、CSV 导出和疑似来源页共享的筛选条件。 */
export interface EventFilters {
  /** range 是后端支持的有限时间范围。 */
  range: EventRange
  /** diskId 是可选的服务端硬盘标识。 */
  diskId: string
  /** type 是可选事件类型枚举。 */
  type: string
  /** confidence 是可选可信度筛选。 */
  confidence: Confidence
  /** source 是可选疑似来源关键词。 */
  source: string
}
/** Overview 是总览页使用的真实聚合状态。 */
export interface Overview {
  /** mechanicalDisks 是已识别机械硬盘数量。 */
  mechanicalDisks: number
  /** activeDisks 是当前活动或空闲机械盘数量。 */
  activeDisks: number
  /** standbyDisks 是连续确认后的待机机械盘数量。 */
  standbyDisks: number
  /** unknownDisks 是当前无法可靠判断状态的机械盘数量。 */
  unknownDisks: number
  /** todayWakeups 是今日已持久化唤醒次数。 */
  todayWakeups: number
  /** lastWakeupAt 是最近一次唤醒时间。 */
  lastWakeupAt?: string
  /** suspectedSource 是最近事件已有的疑似来源摘要。 */
  suspectedSource: string
  /** collectorHealthy 表示采集服务最近是否健康。 */
  collectorHealthy: boolean
  /** databaseStatus 是数据库健康摘要。 */
  databaseStatus: string
  /** lastRefreshAt 是后端数据最近刷新时间。 */
  lastRefreshAt?: string
}
/** Settings 是后端再次校验的采样、保留和展示设置。 */
export interface Settings {
  /** sampleIntervalSeconds 是采样间隔，单位为秒。 */
  sampleIntervalSeconds: number
  /** stateConfirmations 是状态防抖确认次数。 */
  stateConfirmations: number
  /** retentionDays 是历史事件保留天数。 */
  retentionDays: number
  /** maxDatabaseMB 是数据库体积上限，单位为 MB。 */
  maxDatabaseMB: number
  /** logLevel 是滚动日志级别。 */
  logLevel: string
  /** logRetentionFiles 是滚动日志保留文件数。 */
  logRetentionFiles: number
  /** recordLowConfidence 表示是否保留低可信度来源。 */
  recordLowConfidence: boolean
  /** showMaskedSerial 表示是否显示后端脱敏序列号。 */
  showMaskedSerial: boolean
  /** defaultTimeRange 是事件页默认时间范围。 */
  defaultTimeRange: EventRange
  /** ignoredProcesses 是需要排除的精确进程名称列表。 */
  ignoredProcesses: string[]
  /** enableHdparmProbe 表示是否启用实验性固定参数探测。 */
  enableHdparmProbe: boolean
}
/** VersionInfo 是构建时注入并由版本接口返回的真实版本信息。 */
export interface VersionInfo {
  /** version 是 VERSION 注入的应用版本。 */
  version: string
  /** commit 是可选构建提交号。 */
  commit?: string
  /** buildTime 是可选构建时间。 */
  buildTime?: string
  /** platform 是可选目标平台。 */
  platform?: string
}
/** DiagnosticApplication 是诊断报告内的应用构建信息。 */
export interface DiagnosticApplication {
  /** version 是诊断进程报告的应用版本。 */
  version?: string
  /** commit 是诊断进程报告的提交号。 */
  commit?: string
  /** buildTime 是诊断进程报告的构建时间。 */
  buildTime?: string
  /** platform 是诊断进程报告的目标平台。 */
  platform?: string
}
/** DiagnosticReport 是现有诊断接口返回的脱敏系统状态。 */
export interface DiagnosticReport {
  /** generatedAt 是报告生成时间。 */
  generatedAt?: string
  /** fnosVersion 是官方环境提供的 fnOS 版本。 */
  fnosVersion?: string
  /** kernelVersion 是有限内核版本摘要。 */
  kernelVersion?: string
  /** architecture 是 CPU 架构。 */
  architecture?: string
  /** application 是后端构建信息。 */
  application?: DiagnosticApplication
  /** runUser 是后端脱敏后的运行用户。 */
  runUser?: string
  /** serverStatus 是 Server 健康摘要。 */
  serverStatus?: string
  /** collectorStatus 是 Collector 健康摘要。 */
  collectorStatus?: string
  /** gatewaySocketStatus 是统一网关 Socket 摘要。 */
  gatewaySocketStatus?: string
  /** collectorSocketStatus 是内部 Socket 摘要。 */
  collectorSocketStatus?: string
  /** databaseStatus 是 SQLite 健康摘要。 */
  databaseStatus?: string
  /** databaseSizeBytes 是数据库体积，单位为字节。 */
  databaseSizeBytes?: number
  /** schemaVersion 是当前数据库迁移版本。 */
  schemaVersion?: number
  /** disks 是诊断时的脱敏设备扫描结果。 */
  disks?: Disk[]
  /** availableCommands 只表示固定命令是否存在。 */
  availableCommands?: Record<string, boolean>
  /** permissionChecks 是有限权限检查说明。 */
  permissionChecks?: string[]
  /** recentErrors 是经过凭据过滤的有限错误摘要。 */
  recentErrors?: string[]
  /** settings 是当前非敏感配置。 */
  settings?: Settings
  /** versionConsistent 表示运行版本是否由发布构建注入。 */
  versionConsistent?: boolean
}
/** SourceRow 是从当前事件页聚合出的有限疑似来源摘要。 */
export interface SourceRow {
  /** source 是当前页事件中出现的疑似来源名称。 */
  source: string
  /** type 是由实际命中字段确定的来源类型。 */
  type: string
  /** count 是来源在当前事件页出现次数。 */
  count: number
  /** confidence 是该来源最近有效证据的可信度。 */
  confidence: Confidence
  /** reason 是后端保留的判断依据。 */
  reason: string
}

/** request 调用统一网关 API，失败时抛出后端可读错误并阻止旧数据伪装为最新。 */
export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers || {}) },
  })
  const body = await response.json().catch(() => ({})) as { error?: string }
  if (!response.ok) throw new Error(body.error || `请求失败 (${response.status})`)
  return body as T
}

/** formatTime 使用浏览器本地时区格式化 RFC3339 时间，空值和无效值统一降级为短横线。 */
export function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString()
}

/** formatBytes 以二进制单位格式化字节数，不为缺失数据生成虚假精度。 */
export function formatBytes(value = 0): string {
  if (value < 1024) return `${value} B`
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MB`
  if (value < 1024 ** 4) return `${(value / 1024 ** 3).toFixed(1)} GB`
  return `${(value / 1024 ** 4).toFixed(2)} TB`
}

/** buildEventParams 将页面全部筛选条件映射到后端支持的查询参数。 */
export function buildEventParams(filters: EventFilters, page?: number, pageSize?: number): URLSearchParams {
  const params = new URLSearchParams({ range: filters.range })
  if (filters.diskId) params.set('diskId', filters.diskId)
  if (filters.type) params.set('type', filters.type)
  if (filters.confidence) params.set('confidence', filters.confidence)
  if (filters.source.trim()) params.set('source', filters.source.trim())
  if (page !== undefined) params.set('page', String(page))
  if (pageSize !== undefined) params.set('pageSize', String(pageSize))
  return params
}

/** eventSource 按 fnOS 应用、进程、容器的现有字段顺序返回可展示疑似来源。 */
export function eventSource(event: WakeEvent): string {
  return event.suspectFnosApp || event.suspectProcess || event.suspectDockerContainer || ''
}

/** sourceType 根据实际命中的来源字段返回类型，不从名称猜测来源类别。 */
export function sourceType(event: WakeEvent): string {
  if (event.suspectFnosApp) return 'fnOS 应用'
  if (event.suspectProcess) return '进程'
  if (event.suspectDockerContainer) return 'Docker 容器'
  return '未知类型'
}

/** aggregateSources 只汇总当前事件页的真实证据，不把有限分页描述成完整历史排行。 */
export function aggregateSources(events: WakeEvent[]): SourceRow[] {
  const rows = new Map<string, SourceRow>()
  for (const event of events) {
    const source = eventSource(event)
    if (!source || source.includes('fn-disk-wakeup-')) continue
    const previous = rows.get(source)
    rows.set(source, {
      source,
      type: sourceType(event),
      count: (previous?.count || 0) + 1,
      confidence: event.confidence || previous?.confidence || '低',
      reason: event.reason || previous?.reason || '当前事件未提供更多判断依据',
    })
  }
  return [...rows.values()].sort((left, right) => right.count - left.count)
}

/** capabilityLabel 把内部探测方法转换为用户可读说明，同时保留原值供 title 提示。 */
export function capabilityLabel(disk: Disk): string {
  if (!disk.rotational) return '不支持（非机械介质）'
  if (!disk.capabilitySupported) return '当前无法无损探测'
  if (disk.detectionMethod === 'hdparm-C') return 'hdparm -C（实验性）'
  return disk.detectionMethod || '支持状态探测'
}

/** eventTypeLabel 将后端固定事件枚举转换为中文，不把服务事件伪装成磁盘状态。 */
export const eventTypeLabel: Record<string, string> = {
  disk_wakeup: '硬盘唤醒',
  disk_standby: '进入待机',
  disk_activity: '状态活动',
  state_unknown: '状态未知',
  collector_offline: 'Collector 离线',
  collector_recovered: 'Collector 恢复',
}

/** stateLabel 保持磁盘状态中文语义，其中 unknown 与 standby 始终分离。 */
export const stateLabel: Record<DiskState, string> = {
  active: '活动', idle: '空闲', standby: '待机', unknown: '未知', unsupported: '不支持',
}

/** rangeLabel 将有限时间范围转换为页面说明。 */
export const rangeLabel: Record<EventRange, string> = { '24h': '近 24 小时', '7d': '近 7 天', '30d': '近 30 天' }
