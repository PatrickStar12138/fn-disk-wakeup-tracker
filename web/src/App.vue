<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { API_BASE, aggregateSources, buildEventParams, request, type DiagnosticReport, type Disk, type EventFilters, type EventPage, type NavigationItem, type Overview, type Page, type Settings, type VersionInfo, type WakeEvent } from './api'
import AppHeader from './components/AppHeader.vue'
import AppSidebar from './components/AppSidebar.vue'
import ErrorState from './components/ErrorState.vue'
import LoadingSkeleton from './components/LoadingSkeleton.vue'
import GlobalStatusBar from './components/GlobalStatusBar.vue'
import ToastMessage from './components/ToastMessage.vue'
import AboutPage from './pages/AboutPage.vue'
import DiagnosticsPage from './pages/DiagnosticsPage.vue'
import DisksPage from './pages/DisksPage.vue'
import EventsPage from './pages/EventsPage.vue'
import OverviewPage from './pages/OverviewPage.vue'
import SettingsPage from './pages/SettingsPage.vue'
import SourcesPage from './pages/SourcesPage.vue'

const navigation: NavigationItem[] = [
  { id: 'overview', label: '总览', icon: 'home' },
  { id: 'disks', label: '硬盘', icon: 'disk' },
  { id: 'events', label: '唤醒事件', icon: 'events' },
  { id: 'sources', label: '疑似来源', icon: 'sources' },
  { id: 'diagnostics', label: '诊断', icon: 'diagnostics' },
  { id: 'settings', label: '设置', icon: 'settings' },
  { id: 'about', label: '关于', icon: 'info' },
]

const subtitles: Record<Page, string> = {
  overview: '查看真实采集状态、唤醒趋势和最近证据',
  disks: '物理磁盘状态与无损探测能力',
  events: '按后端分页查看状态事件和判断依据',
  sources: '基于当前事件筛选结果汇总',
  diagnostics: '查看脱敏运行环境与能力报告',
  settings: '调整采集、保留和展示选项',
  about: '应用能力、构建信息和已知限制',
}

const current = ref<Page>('overview')
const loading = ref(true)
const eventsLoading = ref(false)
const refreshing = ref(false)
const saving = ref(false)
const menuOpen = ref(false)
const error = ref('')
const overview = ref<Overview>()
const disks = ref<Disk[]>([])
const events = ref<WakeEvent[]>([])
const eventTotal = ref(0)
const eventPage = ref(1)
const pageSize = 20
const settings = ref<Settings>()
const diagnostics = ref<DiagnosticReport>()
const version = ref<VersionInfo>({ version: __APP_VERSION__ })
const filters = reactive<EventFilters>({ range: '24h', diskId: '', type: '', confidence: '', source: '' })
const toast = reactive({ visible: false, message: '', kind: 'success' as 'success' | 'error' })
const icon64 = `${import.meta.env.BASE_URL}icon_64.png`
const icon256 = `${import.meta.env.BASE_URL}icon_256.png`
const requestController = new AbortController()
let toastTimer = 0
let initialSettingsApplied = false

/** pageTitle 从固定导航表返回当前标题，避免页面组件各自维护重复名称。 */
const pageTitle = computed(() => navigation.find((item) => item.id === current.value)?.label || '')
/** sourceRows 仅从当前后端事件页汇总疑似来源，不暗示全量历史统计。 */
const sourceRows = computed(() => aggregateSources(events.value))
/** eventExportURL 复用列表的全部筛选参数，保证 CSV 与页面条件一致。 */
const eventExportURL = computed(() => `${API_BASE}/events/export.csv?${buildEventParams(filters).toString()}`)

/** showToast 展示可访问的平滑提示，并在停留后触发退出动画。 */
function showToast(message: string, kind: 'success' | 'error' = 'success'): void {
  window.clearTimeout(toastTimer)
  toast.message = message
  toast.kind = kind
  toast.visible = true
  toastTimer = window.setTimeout(() => { toast.visible = false }, 3600)
}

/** readableError 将未知异常转换为用户可读文本，同时不暴露内部堆栈。 */
function readableError(cause: unknown, fallback: string): string {
  return cause instanceof Error && cause.message ? cause.message : fallback
}

/** clearPrimaryData 清除加载失败前的旧数据，防止页面把过期信息显示成最新结果。 */
function clearPrimaryData(): void {
  overview.value = undefined
  disks.value = []
  events.value = []
  eventTotal.value = 0
}

/** loadEvents 按当前全部筛选条件请求一个有上限事件页，失败时清除旧事件。 */
async function loadEvents(page = eventPage.value): Promise<void> {
  eventsLoading.value = true
  try {
    const params = buildEventParams(filters, page, pageSize)
    const result = await request<EventPage>(`/events?${params.toString()}`, { signal: requestController.signal })
    events.value = result.items || []
    eventTotal.value = result.total || 0
    eventPage.value = result.page || page
  } catch (cause) {
    events.value = []
    eventTotal.value = 0
    throw cause
  } finally {
    eventsLoading.value = false
  }
}

/** loadPrimaryData 并行加载公共真实数据，并在首次成功时采用后端默认时间范围。 */
async function loadPrimaryData(): Promise<void> {
  const [overviewResult, diskResult, versionResult, settingsResult] = await Promise.all([
    request<Overview>('/overview', { signal: requestController.signal }),
    request<{ items: Disk[] }>('/disks', { signal: requestController.signal }),
    request<VersionInfo>('/version', { signal: requestController.signal }),
    request<Settings>('/settings', { signal: requestController.signal }),
  ])
  overview.value = overviewResult
  disks.value = diskResult.items || []
  version.value = versionResult
  settings.value = settingsResult
  if (!initialSettingsApplied) {
    filters.range = settingsResult.defaultTimeRange
    initialSettingsApplied = true
  }
  await loadEvents(1)
}

/** loadAll 展示首次骨架屏并协调公共请求，任一失败都显示明确错误状态。 */
async function loadAll(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    await loadPrimaryData()
  } catch (cause) {
    clearPrimaryData()
    error.value = readableError(cause, '页面数据加载失败')
  } finally {
    loading.value = false
  }
}

/** loadDiagnostics 获取现有脱敏诊断报告，失败时不保留旧报告。 */
async function loadDiagnostics(): Promise<void> {
  error.value = ''
  diagnostics.value = undefined
  try {
    diagnostics.value = await request<DiagnosticReport>('/diagnostics', { signal: requestController.signal })
  } catch (cause) {
    error.value = readableError(cause, '诊断报告加载失败')
  }
}

/** refresh 请求 Collector 在下一安全采样周期刷新，并重新读取当前真实数据。 */
async function refresh(): Promise<void> {
  if (refreshing.value) return
  refreshing.value = true
  error.value = ''
  try {
    await request<{ ok?: boolean }>('/refresh', { method: 'POST', signal: requestController.signal })
    await loadPrimaryData()
    if (current.value === 'diagnostics') await loadDiagnostics()
    showToast('已请求下一安全采样周期，并更新当前数据')
  } catch (cause) {
    clearPrimaryData()
    error.value = readableError(cause, '刷新失败')
    showToast(error.value, 'error')
  } finally {
    refreshing.value = false
  }
}

/** selectPage 切换固定页面；诊断页按需读取报告，其余页面复用公共数据。 */
async function selectPage(page: Page): Promise<void> {
  current.value = page
  menuOpen.value = false
  error.value = ''
  if (page === 'diagnostics') await loadDiagnostics()
}

/** retryCurrent 根据当前页面重试对应请求，避免诊断错误跳回总览。 */
async function retryCurrent(): Promise<void> {
  if (current.value === 'diagnostics') await loadDiagnostics()
  else await loadAll()
}

/** applyEventFilters 保存页面传回的全部条件，并从第一页重新请求真实事件。 */
async function applyEventFilters(next: EventFilters): Promise<void> {
  Object.assign(filters, next)
  error.value = ''
  try {
    await loadEvents(1)
  } catch (cause) {
    error.value = readableError(cause, '事件筛选失败')
  }
}

/** reloadEvents 使用当前筛选条件重新加载当前页，失败时显示错误状态。 */
async function reloadEvents(): Promise<void> {
  error.value = ''
  try {
    await loadEvents(eventPage.value)
  } catch (cause) {
    error.value = readableError(cause, '事件重新加载失败')
  }
}

/** changeEventPage 在后端分页边界内切换，不一次性加载全部历史事件。 */
async function changeEventPage(delta: number): Promise<void> {
  const next = eventPage.value + delta
  if (next < 1 || (next - 1) * pageSize >= eventTotal.value) return
  error.value = ''
  try {
    await loadEvents(next)
  } catch (cause) {
    error.value = readableError(cause, '事件分页加载失败')
  }
}

/** saveSettings 仅在后端确认后替换已保存设置，失败时保留最近确认值。 */
async function saveSettings(next: Settings): Promise<void> {
  if (saving.value) return
  saving.value = true
  try {
    const result = await request<{ settings: Settings }>('/settings', {
      method: 'PUT',
      body: JSON.stringify(next),
      signal: requestController.signal,
    })
    settings.value = result.settings
    showToast('设置已保存，将在下一采样周期安全重载')
  } catch (cause) {
    showToast(readableError(cause, '设置保存失败'), 'error')
  } finally {
    saving.value = false
  }
}

/** handlePageNotification 转发子页面的成功或失败提示，统一使用平滑 Toast。 */
function handlePageNotification(message: string, kind: 'success' | 'error'): void {
  showToast(message, kind)
}

/** closeAppResources 取消未完成请求和提示计时器，避免 iframe 关闭后的异步更新。 */
function closeAppResources(): void {
  requestController.abort()
  window.clearTimeout(toastTimer)
}

onMounted(loadAll)
onBeforeUnmount(closeAppResources)
</script>

<template>
  <div class="app-shell">
    <AppSidebar :items="navigation" :current="current" :open="menuOpen" :icon-url="icon64" :version="version.version" @navigate="selectPage" @close="menuOpen = false" />
    <main class="app-main">
      <div class="main-scroll">
        <AppHeader :title="pageTitle" :subtitle="subtitles[current]" :updated-at="overview?.lastRefreshAt" :refreshing="refreshing" @refresh="refresh" @menu="menuOpen = !menuOpen" />
        <ToastMessage :visible="toast.visible" :message="toast.message" :kind="toast.kind" />
        <div class="page-content">
          <ErrorState v-if="error" :message="error" @retry="retryCurrent" />
          <LoadingSkeleton v-else-if="loading" />
          <OverviewPage v-else-if="current === 'overview'" :overview="overview" :disks="disks" :events="events" @navigate="selectPage" />
          <DisksPage v-else-if="current === 'disks'" :disks="disks" />
          <EventsPage v-else-if="current === 'events'" :events="events" :disks="disks" :filters="filters" :page="eventPage" :page-size="pageSize" :total="eventTotal" :loading="eventsLoading" :export-url="eventExportURL" @filters="applyEventFilters" @reload="reloadEvents" @page="changeEventPage" />
          <SourcesPage v-else-if="current === 'sources'" :rows="sourceRows" :filters="filters" />
          <DiagnosticsPage v-else-if="current === 'diagnostics'" :report="diagnostics" @notify="handlePageNotification" />
          <SettingsPage v-else-if="current === 'settings' && settings" :settings="settings" :saving="saving" @save="saveSettings" />
          <AboutPage v-else-if="current === 'about'" :version="version" :icon-url="icon256" />
        </div>
      </div>
      <GlobalStatusBar :collector-healthy="overview?.collectorHealthy" :database-status="overview?.databaseStatus" :last-refresh-at="overview?.lastRefreshAt" />
    </main>
  </div>
</template>
