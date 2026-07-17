<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import WakeChart from './components/WakeChart.vue'
import { API_BASE, formatBytes, formatTime, request, stateLabel, type Disk, type EventPage, type Overview, type Settings, type WakeEvent } from './api'

/** Page 是控制台允许切换的固定页面标识。 */
type Page = 'overview' | 'disks' | 'events' | 'sources' | 'diagnostics' | 'settings' | 'about'
const pages: { id: Page; label: string; icon: string }[] = [
  { id: 'overview', label: '总览', icon: '⌂' }, { id: 'disks', label: '硬盘', icon: '◉' },
  { id: 'events', label: '唤醒事件', icon: '↯' }, { id: 'sources', label: '疑似来源', icon: '◎' },
  { id: 'diagnostics', label: '诊断', icon: '＋' }, { id: 'settings', label: '设置', icon: '⚙' },
  { id: 'about', label: '关于', icon: 'i' },
]
const current = ref<Page>('overview')
const loading = ref(true); const refreshing = ref(false); const error = ref(''); const menuOpen = ref(false)
const toast = reactive({ visible: false, message: '', kind: 'success' as 'success' | 'error' }); let toastTimer = 0
const overview = ref<Overview>(); const disks = ref<Disk[]>([]); const events = ref<WakeEvent[]>([])
const eventTotal = ref(0); const eventPage = ref(1); const pageSize = 20
const filters = reactive({ range: '24h', diskId: '', type: '', confidence: '', source: '' })
const settings = ref<Settings>(); const settingsDraft = ref<Settings>(); const saving = ref(false); const diagnostics = ref<Record<string, unknown>>()
const version = ref(__APP_VERSION__)
const icon64 = `${import.meta.env.BASE_URL}icon_64.png`
const icon256 = `${import.meta.env.BASE_URL}icon_256.png`
const requestController = new AbortController()

/** showToast 展示平滑进入提示，并在停留后触发平滑退出。 */
function showToast(message: string, kind: 'success' | 'error' = 'success') { window.clearTimeout(toastTimer); toast.message = message; toast.kind = kind; toast.visible = true; toastTimer = window.setTimeout(() => { toast.visible = false }, 3600) }
/** cloneSettings 创建设置草稿，避免表单直接修改已确认值。 */
function cloneSettings(value: Settings): Settings { return JSON.parse(JSON.stringify(value)) as Settings }
/** loadEvents 按当前筛选条件加载一个有限事件页。 */
async function loadEvents(page = eventPage.value) {
  const q = new URLSearchParams({ range: filters.range, page: String(page), pageSize: String(pageSize) })
  if (filters.diskId) q.set('diskId', filters.diskId); if (filters.type) q.set('type', filters.type); if (filters.confidence) q.set('confidence', filters.confidence); if (filters.source) q.set('source', filters.source)
  const data = await request<EventPage>(`/events?${q}`, { signal: requestController.signal }); events.value = data.items; eventTotal.value = data.total; eventPage.value = data.page
}
/** loadAll 并行加载总览、硬盘和版本，任一失败即显示错误状态。 */
async function loadAll() {
  loading.value = true; error.value = ''
  try {
    const [o, d, v, s] = await Promise.all([request<Overview>('/overview', { signal: requestController.signal }), request<{ items: Disk[] }>('/disks', { signal: requestController.signal }), request<{ version: string }>('/version', { signal: requestController.signal }), request<Settings>('/settings', { signal: requestController.signal })])
    overview.value = o; disks.value = d.items; version.value = v.version; settings.value = s; settingsDraft.value = cloneSettings(s); filters.range = s.defaultTimeRange; await loadEvents(1)
  } catch (e) { error.value = e instanceof Error ? e.message : '页面加载失败' } finally { loading.value = false }
}
/** refresh 防止重复点击，并请求下一安全采样周期后刷新页面。 */
async function refresh() { if (refreshing.value) return; refreshing.value = true; try { await request('/refresh', { method: 'POST', signal: requestController.signal }); await loadAll(); showToast('状态已刷新') } catch (e) { showToast(e instanceof Error ? e.message : '刷新失败', 'error') } finally { refreshing.value = false } }
/** selectPage 切换页面，并按需加载设置或脱敏诊断报告。 */
async function selectPage(page: Page) { current.value = page; menuOpen.value = false; error.value = ''; try { if (page === 'settings' && !settings.value) { settings.value = await request<Settings>('/settings', { signal: requestController.signal }); settingsDraft.value = cloneSettings(settings.value) } if (page === 'diagnostics') diagnostics.value = await request('/diagnostics', { signal: requestController.signal }) } catch (e) { error.value = e instanceof Error ? e.message : '页面加载失败' } }
/** saveSettings 使用 saving 标志防重复提交，并只在后端确认后更新页面。 */
async function saveSettings() { if (!settingsDraft.value || saving.value) return; saving.value = true; try { const result = await request<{ settings: Settings }>('/settings', { method: 'PUT', body: JSON.stringify(settingsDraft.value), signal: requestController.signal }); settings.value = result.settings; settingsDraft.value = cloneSettings(result.settings); showToast('设置已保存并将在下一采样周期安全重载') } catch (e) { showToast(e instanceof Error ? e.message : '设置保存失败', 'error') } finally { saving.value = false } }
/** changePage 在有效边界内切换事件分页。 */
function changePage(delta: number) { const next = eventPage.value + delta; if (next < 1 || (next - 1) * pageSize >= eventTotal.value) return; loadEvents(next).catch((e) => { error.value = e.message }) }
/** sourceRows 从当前真实事件页聚合疑似来源，不把空证据伪造成来源。 */
const sourceRows = computed(() => { const map = new Map<string, { source: string; count: number; confidence: string; reason: string }>(); for (const e of events.value) { const source = e.suspectFnosApp || e.suspectProcess || e.suspectDockerContainer; if (!source) continue; const old = map.get(source); map.set(source, { source, count: (old?.count || 0) + 1, confidence: e.confidence, reason: e.reason }) } return [...map.values()].sort((a, b) => b.count - a.count) })
/** title 返回当前页面的中文标题。 */
const title = computed(() => pages.find((p) => p.id === current.value)?.label || '')
/** ignoredText 在多行文本和进程名称数组之间转换。 */
const ignoredText = computed({ get: () => settingsDraft.value?.ignoredProcesses.join('\n') || '', set: (v: string) => { if (settingsDraft.value) settingsDraft.value.ignoredProcesses = v.split('\n').map((x) => x.trim()).filter(Boolean) } })
onMounted(loadAll)
// 组件卸载时取消全部未完成请求，避免 iframe 关闭后继续更新状态。
onBeforeUnmount(() => requestController.abort())
</script>

<template>
  <div class="shell">
    <aside :class="['sidebar', { open: menuOpen }]">
      <div class="brand"><img :src="icon64" alt="" @error="($event.target as HTMLImageElement).style.display='none'" /><div><strong>硬盘唤醒追踪器</strong><small>Disk observability</small></div></div>
      <nav aria-label="主导航"><button v-for="page in pages" :key="page.id" :class="{ active: current === page.id }" @click="selectPage(page.id)"><span class="nav-icon">{{ page.icon }}</span><span>{{ page.label }}</span></button></nav>
      <div class="sidebar-foot"><span :class="['status-dot', overview?.collectorHealthy ? 'ok' : 'bad']"></span>{{ overview?.collectorHealthy ? '采集服务正常' : '采集服务离线' }}<small>v{{ version }}</small></div>
    </aside>
    <div v-if="menuOpen" class="scrim" @click="menuOpen = false"></div>
    <main>
      <header><div class="headline"><button class="menu-button" aria-label="打开导航" @click="menuOpen = !menuOpen">☰</button><div><p>硬盘监控 / {{ title }}</p><h1>{{ title }}</h1></div></div><div class="header-actions"><span class="updated">最后更新 {{ formatTime(overview?.lastRefreshAt) }}</span><button class="button secondary" :disabled="refreshing" @click="refresh"><span :class="{ spin: refreshing }">↻</span>{{ refreshing ? '刷新中' : '刷新' }}</button></div></header>

      <div v-if="toast.visible" :class="['toast', toast.kind]" role="status"><span>{{ toast.kind === 'success' ? '✓' : '!' }}</span>{{ toast.message }}</div>
      <section v-if="error" class="error-state"><strong>数据加载失败</strong><p>{{ error }}</p><button class="button" @click="loadAll">重试</button></section>
      <section v-else-if="loading" class="skeleton-grid" aria-label="正在加载"><div v-for="n in 8" :key="n" class="skeleton"></div></section>

      <template v-else-if="current === 'overview'">
        <section class="metric-strip">
          <article><span>机械硬盘</span><strong>{{ overview?.mechanicalDisks ?? 0 }}</strong><small>已识别物理盘</small></article>
          <article><span>当前活动</span><strong>{{ overview?.activeDisks ?? 0 }}</strong><small>活动或空闲</small></article>
          <article><span>当前待机</span><strong>{{ overview?.standbyDisks ?? 0 }}</strong><small>连续确认后计入</small></article>
          <article><span>今日唤醒</span><strong>{{ overview?.todayWakeups ?? 0 }}</strong><small>{{ formatTime(overview?.lastWakeupAt) }}</small></article>
        </section>
        <section class="overview-grid">
          <article class="panel disk-panel"><div class="panel-title"><div><span class="eyebrow">实时状态</span><h2>硬盘状态</h2></div><button class="link" @click="selectPage('disks')">查看全部 →</button></div><div v-if="disks.length" class="disk-list"><div v-for="disk in disks.slice(0, 5)" :key="disk.id" class="disk-row"><span :class="['state-orb', disk.state]"></span><div class="disk-name"><strong>/dev/{{ disk.device }}</strong><small>{{ disk.model || '未知型号' }}</small></div><span :class="['badge', disk.state]">{{ stateLabel[disk.state] }}</span><span class="disk-meta">今日 {{ disk.todayWakeups }} 次</span></div></div><div v-else class="empty"><strong>未发现硬盘</strong><small>等待 Collector 完成首次安全扫描</small></div></article>
          <article class="panel"><div class="panel-title"><div><span class="eyebrow">最近 24 小时</span><h2>唤醒趋势</h2></div></div><WakeChart :events="events" /></article>
          <article class="panel recent"><div class="panel-title"><div><span class="eyebrow">事件流</span><h2>最近唤醒事件</h2></div><button class="link" @click="selectPage('events')">完整时间线 →</button></div><div v-if="events.length" class="timeline"><div v-for="item in events.slice(0, 5)" :key="item.id"><span class="timeline-dot"></span><div><strong>/dev/{{ item.device || '—' }} · {{ item.type === 'disk_wakeup' ? '硬盘唤醒' : item.type }}</strong><small>{{ formatTime(item.startedAt) }} · {{ item.suspectFnosApp || item.suspectProcess || '暂无疑似来源' }}</small></div><span :class="['confidence', item.confidence]">{{ item.confidence || '低' }}</span></div></div><div v-else class="empty"><strong>暂无事件</strong><small>不会使用模拟数据填充时间线</small></div></article>
          <article class="panel insight"><span class="eyebrow">能力提示</span><h2>低干扰监测已启用</h2><p>无法确认无损探测能力时显示“未知”；SSD/NVMe 显示“不支持”。疑似来源只表示时间相关性，不代表确定因果关系。</p><div class="source-callout"><span>当前疑似来源</span><strong>{{ overview?.suspectedSource || '暂无证据' }}</strong></div></article>
        </section>
      </template>

      <section v-else-if="current === 'disks'" class="panel full"><div class="panel-title"><div><span class="eyebrow">物理设备</span><h2>硬盘与能力</h2></div></div><div class="table-wrap"><table><thead><tr><th>设备</th><th>型号 / 序列号</th><th>容量 / 总线</th><th>当前状态</th><th>上次变化</th><th>检测能力</th></tr></thead><tbody><tr v-for="disk in disks" :key="disk.id"><td><strong>/dev/{{ disk.device }}</strong><small>{{ disk.rotational ? '机械盘' : '非机械介质' }}</small></td><td>{{ disk.model || '未知型号' }}<small>{{ disk.maskedSerial || '无序列号' }}</small></td><td>{{ formatBytes(disk.capacityBytes) }}<small>{{ disk.busType }}</small></td><td><span :class="['badge', disk.state]">{{ stateLabel[disk.state] }}</span><small>前次：{{ stateLabel[disk.previousState] }}</small></td><td>{{ formatTime(disk.lastStateChange) }}<small>今日唤醒 {{ disk.todayWakeups }} 次</small></td><td>{{ disk.capabilitySupported ? '支持' : '受限' }}<small>{{ disk.detectionMethod }}</small></td></tr></tbody></table></div><div v-if="!disks.length" class="empty"><strong>暂无设备数据</strong></div></section>

      <section v-else-if="current === 'events'" class="panel full"><div class="panel-title"><div><span class="eyebrow">可筛选时间线</span><h2>唤醒事件</h2></div><a class="button secondary" :href="`${API_BASE}/events/export.csv?range=${filters.range}`">导出 CSV</a></div><div class="filters"><select v-model="filters.range" @change="loadEvents(1)"><option value="24h">最近 24 小时</option><option value="7d">最近 7 天</option><option value="30d">最近 30 天</option></select><select v-model="filters.diskId" @change="loadEvents(1)"><option value="">全部硬盘</option><option v-for="d in disks" :key="d.id" :value="d.id">/dev/{{ d.device }}</option></select><select v-model="filters.type" @change="loadEvents(1)"><option value="">全部事件</option><option value="disk_wakeup">硬盘唤醒</option><option value="disk_standby">进入待机</option><option value="disk_activity">状态活动</option><option value="state_unknown">状态未知</option></select><select v-model="filters.confidence" @change="loadEvents(1)"><option value="">全部可信度</option><option>高</option><option>中</option><option>低</option></select><input v-model="filters.source" placeholder="筛选疑似来源" @keyup.enter="loadEvents(1)" /></div><div class="table-wrap"><table><thead><tr><th>时间 / 设备</th><th>状态变化</th><th>I/O 增量</th><th>疑似来源</th><th>判断依据</th><th>可信度</th></tr></thead><tbody><tr v-for="item in events" :key="item.id"><td>{{ formatTime(item.startedAt) }}<small>/dev/{{ item.device || '—' }}</small></td><td>{{ stateLabel[item.fromState] }} → {{ stateLabel[item.toState] }}<small>{{ item.type }}</small></td><td>读 {{ formatBytes(item.readDelta) }}<small>写 {{ formatBytes(item.writeDelta) }}</small></td><td>{{ item.suspectFnosApp || item.suspectProcess || item.suspectDockerContainer || '暂无证据' }}</td><td class="reason">{{ item.reason || '未形成有效判断依据' }}</td><td><span :class="['confidence', item.confidence]">{{ item.confidence || '低' }}</span></td></tr></tbody></table></div><div v-if="!events.length" class="empty"><strong>筛选范围内没有事件</strong><small>不会把 unknown 伪装成 standby，也不会生成假曲线</small></div><div class="pagination"><button :disabled="eventPage <= 1" @click="changePage(-1)">上一页</button><span>第 {{ eventPage }} 页 · 共 {{ eventTotal }} 条</span><button :disabled="eventPage * pageSize >= eventTotal" @click="changePage(1)">下一页</button></div></section>

      <section v-else-if="current === 'sources'" class="panel full"><div class="panel-title"><div><span class="eyebrow">相关性分析</span><h2>疑似来源排行</h2></div></div><div class="notice">以下结论来自状态变化前后的 I/O 时间相关性，并非确定因果关系。应用自身 I/O 会单独标记并默认排除。</div><div v-if="sourceRows.length" class="source-list"><article v-for="row in sourceRows" :key="row.source"><div><strong>{{ row.source }}</strong><small>{{ row.reason }}</small></div><span>{{ row.count }} 次</span><span :class="['confidence', row.confidence]">{{ row.confidence }}</span></article></div><div v-else class="empty"><strong>暂无可归因证据</strong><small>只有具备进程 I/O 增量和时间窗口依据时才展示疑似来源</small></div></section>

      <section v-else-if="current === 'diagnostics'" class="panel full"><div class="panel-title"><div><span class="eyebrow">一键诊断</span><h2>运行环境与能力报告</h2></div><div class="button-row"><a class="button secondary" :href="`${API_BASE}/diagnostics.txt`">下载文本</a><a class="button secondary" :href="`${API_BASE}/diagnostics.json`">下载 JSON</a></div></div><div class="notice">报告自动脱敏序列号和用户名，不输出 Token、密码、Cookie、完整环境变量或任意文件内容。</div><pre class="diagnostic-preview">{{ JSON.stringify(diagnostics, null, 2) }}</pre></section>

      <section v-else-if="current === 'settings' && settingsDraft" class="panel full settings"><div class="panel-title"><div><span class="eyebrow">采集与保留</span><h2>设置</h2></div><button class="button" :disabled="saving" @click="saveSettings">{{ saving ? '保存中…' : '保存设置' }}</button></div><div class="form-grid"><label>采样间隔（5–300 秒）<input v-model.number="settingsDraft.sampleIntervalSeconds" type="number" min="5" max="300" /></label><label>状态确认次数（1–10）<input v-model.number="settingsDraft.stateConfirmations" type="number" min="1" max="10" /></label><label>事件保留天数（1–365）<input v-model.number="settingsDraft.retentionDays" type="number" min="1" max="365" /></label><label>数据库上限（20–2048 MB）<input v-model.number="settingsDraft.maxDatabaseMB" type="number" min="20" max="2048" /></label><label>日志级别<select v-model="settingsDraft.logLevel"><option>error</option><option>warn</option><option>info</option><option>debug</option></select></label><label>日志保留数量（1–20）<input v-model.number="settingsDraft.logRetentionFiles" type="number" min="1" max="20" /></label><label>默认时间范围<select v-model="settingsDraft.defaultTimeRange"><option value="24h">最近 24 小时</option><option value="7d">最近 7 天</option><option value="30d">最近 30 天</option></select></label><label class="wide">忽略进程列表（每行一个）<textarea v-model="ignoredText" rows="5"></textarea></label></div><div class="switches"><label><input v-model="settingsDraft.enableHdparmProbe" type="checkbox" /> 启用固定参数 hdparm 状态探测（需真机验证控制器行为）</label><label><input v-model="settingsDraft.recordLowConfidence" type="checkbox" /> 记录低可信度来源</label><label><input v-model="settingsDraft.showMaskedSerial" type="checkbox" /> 显示脱敏序列号</label></div></section>

      <section v-else-if="current === 'about'" class="about panel full"><img :src="icon256" alt="硬盘唤醒追踪器图标" /><span class="eyebrow">fnOS 原生应用</span><h2>硬盘唤醒追踪器</h2><strong>版本 {{ version }}</strong><p>以低干扰方式记录机械硬盘状态变化，并提供有证据、带可信度的疑似来源分析。应用不会修改系统休眠、SMART、systemd 或硬盘待机策略。</p><div class="notice">当前版本的无损探测行为、系统分区安装和卸载保留语义仍需在目标 fnOS 设备上完成真机验收。</div></section>
    </main>
  </div>
</template>
