export const API_BASE = '/app/fn-disk-wakeup-tracker/api/v1'

/** DiskState 是前后端共享的磁盘状态，unknown 不等同于 standby。 */
export type DiskState = 'active' | 'idle' | 'standby' | 'unknown' | 'unsupported'
/** Disk 描述硬盘列表中的物理盘身份、能力和最近状态。 */
export interface Disk {
  id: string; device: string; model: string; maskedSerial: string; capacityBytes: number; busType: string
  rotational: boolean; state: DiskState; previousState: DiskState; lastStateChange: string; todayWakeups: number
  lastActivity?: string; detectionMethod: string; capabilitySupported: boolean; present: boolean
}
/** WakeEvent 描述带证据摘要的状态或唤醒事件。 */
export interface WakeEvent {
  id: number; diskId: string; device: string; type: string; fromState: DiskState; toState: DiskState
  startedAt: string; durationMs: number; readDelta: number; writeDelta: number; suspectProcess: string
  suspectFnosApp: string; suspectDockerContainer: string; reason: string; confidence: string; note: string
}
/** EventPage 是有上限的事件分页响应。 */
export interface EventPage { items: WakeEvent[]; page: number; pageSize: number; total: number }
/** Overview 是总览页使用的聚合状态。 */
export interface Overview {
  mechanicalDisks: number; activeDisks: number; standbyDisks: number; unknownDisks: number; todayWakeups: number
  lastWakeupAt?: string; suspectedSource: string; collectorHealthy: boolean; databaseStatus: string; lastRefreshAt?: string
}
/** Settings 是后端再次校验的采样、保留和展示设置。 */
export interface Settings {
  sampleIntervalSeconds: number; stateConfirmations: number; retentionDays: number; maxDatabaseMB: number
  logLevel: string; logRetentionFiles: number; recordLowConfidence: boolean; showMaskedSerial: boolean
  defaultTimeRange: string; ignoredProcesses: string[]; enableHdparmProbe: boolean
}

/** request 调用统一网关 API，失败时抛出后端可读错误并阻止旧数据伪装为最新。 */
export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers || {}) },
  })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(body.error || `请求失败 (${response.status})`)
  return body as T
}

/** formatTime 使用浏览器本地时区格式化 RFC3339 时间。 */
export function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString()
}
/** formatBytes 以二进制单位格式化字节数。 */
export function formatBytes(value = 0): string {
  if (value < 1024) return `${value} B`
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MB`
  return `${(value / 1024 ** 3).toFixed(1)} GB`
}
export const stateLabel: Record<DiskState, string> = { active: '活动', idle: '空闲', standby: '待机', unknown: '未知', unsupported: '不支持' }
