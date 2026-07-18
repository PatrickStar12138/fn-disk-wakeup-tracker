import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import App from './App.vue'

// 在 jsdom 中替换 Canvas 图表实现，只验证真实数据和空状态，不引入原生 canvas 依赖。
vi.mock('uplot', () => ({
  default: class MockUPlot {
    /** destroy 模拟图表释放接口，测试替身没有外部资源需要回收。 */
    destroy(): void { /* 测试替身无需释放浏览器资源。 */ }
  },
}))

const settings = {
  sampleIntervalSeconds: 15,
  stateConfirmations: 3,
  retentionDays: 30,
  maxDatabaseMB: 200,
  logLevel: 'info',
  logRetentionFiles: 5,
  recordLowConfidence: true,
  showMaskedSerial: true,
  defaultTimeRange: '24h',
  ignoredProcesses: ['tracker-helper'],
  enableHdparmProbe: false,
}

const disks = [
  { id: '1', device: 'sda', model: 'Test HDD', maskedSerial: '****1234', capacityBytes: 4_000_000_000, busType: 'sata', rotational: true, state: 'unknown', previousState: 'active', lastStateChange: '2026-07-18T08:00:00Z', todayWakeups: 0, detectionMethod: 'capability_unavailable', capabilitySupported: false, present: true },
  { id: '2', device: 'nvme0n1', model: 'Test SSD', maskedSerial: '', capacityBytes: 2_000_000_000, busType: 'nvme', rotational: false, state: 'unsupported', previousState: 'unsupported', lastStateChange: '2026-07-18T08:00:00Z', todayWakeups: 0, detectionMethod: 'media_type', capabilitySupported: false, present: true },
]

const wakeEvent = {
  id: 1, diskId: '1', device: 'sda', type: 'disk_wakeup', fromState: 'standby', toState: 'active', startedAt: '2026-07-18T08:00:00Z', durationMs: 0, readDelta: 1024, writeDelta: 2048,
  suspectProcess: 'backup-worker', suspectFnosApp: '', suspectDockerContainer: '', reason: '状态变化前 5 秒内检测到有限 I/O 增量', confidence: '中', note: '',
}

const diagnostics = {
  generatedAt: '2026-07-18T08:10:00Z', fnosVersion: '1.2.3', kernelVersion: '6.1-test', architecture: 'amd64',
  application: { version: '0.1.0', commit: 'abc123', buildTime: '2026-07-18T07:00:00Z', platform: 'x86' },
  serverStatus: '正常', collectorStatus: '正常', databaseStatus: '正常', databaseSizeBytes: 4096, schemaVersion: 3,
  availableCommands: { hdparm: true, smartctl: false }, versionConsistent: true,
}

/** response 构造不访问网络的最小 Fetch 响应，并允许测试显式模拟错误状态。 */
function response(body: unknown, ok = true): Promise<Response> {
  return Promise.resolve({ ok, status: ok ? 200 : 500, json: () => Promise.resolve(body) } as Response)
}

/** baseFetch 按现有 API 路径返回可重复的真实形状夹具，不包含参考图演示数据。 */
function baseFetch(url: string | URL | Request, init?: RequestInit): Promise<Response> {
  const target = String(url)
  if (target.endsWith('/overview')) return response({ mechanicalDisks: 1, activeDisks: 0, standbyDisks: 0, unknownDisks: 1, todayWakeups: 0, suspectedSource: '', collectorHealthy: true, databaseStatus: '正常', lastRefreshAt: '2026-07-18T08:00:00Z' })
  if (target.endsWith('/disks')) return response({ items: disks })
  if (target.includes('/events')) return response({ items: [], page: 1, pageSize: 20, total: 0 })
  if (target.endsWith('/version')) return response({ version: '0.1.0', commit: 'abc123', buildTime: '2026-07-18T07:00:00Z', platform: 'x86' })
  if (target.endsWith('/settings')) return response(init?.method === 'PUT' ? { settings: JSON.parse(String(init.body)) } : settings)
  if (target.endsWith('/diagnostics')) return response(diagnostics)
  if (target.endsWith('/refresh')) return response({ ok: true })
  return response({})
}

/** mountApp 挂载控制台并等待首次并行请求结束，供交互测试复用。 */
async function mountApp(): Promise<VueWrapper> {
  const wrapper = mount(App)
  await flushPromises()
  return wrapper
}

/** navigate 使用固定导航文字切换页面，并等待按需诊断请求完成。 */
async function navigate(wrapper: VueWrapper, label: string): Promise<void> {
  const button = wrapper.findAll('nav button').find((item) => item.text().includes(label))
  if (!button) throw new Error(`缺少导航：${label}`)
  await button.trigger('click')
  await flushPromises()
}

describe('App', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn(baseFetch))
    vi.stubGlobal('ResizeObserver', class {
      /** observe 模拟浏览器尺寸监听注册，组件行为由断言直接验证。 */
      observe(): void { /* jsdom 无实际布局。 */ }
      /** disconnect 模拟组件卸载时取消尺寸监听。 */
      disconnect(): void { /* jsdom 无实际监听。 */ }
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  // 测试七个固定导航入口：逐项切换后页面标题应与入口一致，防止拆分页面遗漏挂载。
  it('可切换全部七个页面', async () => {
    const wrapper = await mountApp()
    for (const label of ['总览', '硬盘', '唤醒事件', '疑似来源', '诊断', '设置', '关于']) {
      await navigate(wrapper, label)
      expect(wrapper.get('.app-header h1').text()).toBe(label)
    }
  })

  // 测试未知磁盘：后端返回 unknown 时必须展示“未知”，不得降级成待机。
  it('将 unknown 显示为未知', async () => {
    const wrapper = await mountApp()
    await navigate(wrapper, '硬盘')
    expect(wrapper.text()).toContain('未知')
  })

  // 测试非机械介质：后端返回 unsupported 时必须展示“不支持”。
  it('将 unsupported 显示为不支持', async () => {
    const wrapper = await mountApp()
    await navigate(wrapper, '硬盘')
    expect(wrapper.text()).toContain('不支持')
  })

  // 测试空事件趋势：没有真实 disk_wakeup 时应显示空状态且不创建图表容器。
  it('无事件时不生成假曲线', async () => {
    const wrapper = await mountApp()
    expect(wrapper.text()).toContain('暂无趋势数据')
    expect(wrapper.find('.chart-host').exists()).toBe(false)
  })

  // 测试公共 API 失败：页面应清除数据并展示后端错误，防止旧内容伪装为最新。
  it('API 失败时显示明确错误状态', async () => {
    vi.stubGlobal('fetch', vi.fn(() => response({ error: '网关错误' }, false)))
    const wrapper = await mountApp()
    expect(wrapper.text()).toContain('数据加载失败')
    expect(wrapper.text()).toContain('网关错误')
  })

  // 测试设置成功：后端确认 PUT 后才显示成功 Toast。
  it('设置保存成功后显示 Toast', async () => {
    const wrapper = await mountApp()
    await navigate(wrapper, '设置')
    await wrapper.get('.settings-form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('设置已保存')
  })

  // 测试设置失败：PUT 失败时只能显示错误，不得出现成功文案。
  it('设置保存失败时不伪装成功', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string | URL | Request, init?: RequestInit) => String(url).endsWith('/settings') && init?.method === 'PUT' ? response({ error: '配置目录只读' }, false) : baseFetch(url, init)))
    const wrapper = await mountApp()
    await navigate(wrapper, '设置')
    await wrapper.get('.settings-form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('配置目录只读')
    expect(wrapper.text()).not.toContain('设置已保存，将在')
  })

  // 测试恢复当前值：编辑草稿后应恢复最近一次 GET 成功值，而不是生成前端默认值。
  it('恢复当前值使用最后确认设置', async () => {
    const wrapper = await mountApp()
    await navigate(wrapper, '设置')
    const input = wrapper.get('input[type="number"]')
    await input.setValue('99')
    await wrapper.findAll('button').find((button) => button.text().includes('恢复当前值'))!.trigger('click')
    expect((input.element as HTMLInputElement).value).toBe('15')
  })

  // 测试 CSV 映射：页面设置的五个筛选条件必须全部进入导出链接。
  it('CSV 导出包含全部当前筛选条件', async () => {
    const wrapper = await mountApp()
    await navigate(wrapper, '唤醒事件')
    await wrapper.find('.segmented button:nth-child(2)').trigger('click')
    await wrapper.get('select[data-test="event-type-filter"]').setValue('collector_offline')
    const selects = wrapper.findAll('.filter-layout select')
    await selects[0].setValue('1')
    await selects[2].setValue('高')
    const source = wrapper.get('.source-filter input')
    await source.setValue('backup')
    await source.trigger('keyup.enter')
    await flushPromises()
    const href = wrapper.get('a[download]').attributes('href')
    expect(href).toContain('range=7d')
    expect(href).toContain('diskId=1')
    expect(href).toContain('type=collector_offline')
    expect(href).toContain(`confidence=${encodeURIComponent('高')}`)
    expect(href).toContain('source=backup')
  })

  // 测试 Collector 服务事件筛选：离线和恢复枚举都必须在现有事件类型选择器中可用。
  it('支持筛选 Collector 离线和恢复事件', async () => {
    const wrapper = await mountApp()
    await navigate(wrapper, '唤醒事件')
    const options = wrapper.get('select[data-test="event-type-filter"]').findAll('option').map((option) => option.attributes('value'))
    expect(options).toContain('collector_offline')
    expect(options).toContain('collector_recovered')
  })

  // 测试诊断页面：状态卡和环境摘要必须来自 diagnostics 接口真实字段。
  it('诊断页显示真实诊断字段', async () => {
    const wrapper = await mountApp()
    await navigate(wrapper, '诊断')
    expect(wrapper.text()).toContain('6.1-test')
    expect(wrapper.text()).toContain('Schema 版本')
    expect(wrapper.text()).toContain('amd64')
  })

  // 测试剪贴板失败：浏览器拒绝复制时应展示明确错误 Toast。
  it('复制诊断失败时显示提示', async () => {
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } })
    const wrapper = await mountApp()
    await navigate(wrapper, '诊断')
    await wrapper.findAll('button').find((button) => button.text().includes('复制'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('复制失败')
  })

  // 测试关于页面：版本、提交、构建时间和平台均应使用版本接口返回值。
  it('关于页使用真实版本字段', async () => {
    const wrapper = await mountApp()
    await navigate(wrapper, '关于')
    expect(wrapper.text()).toContain('v0.1.0')
    expect(wrapper.text()).toContain('abc123')
    expect(wrapper.text()).toContain('x86')
  })

  // 测试窄窗口导航：768px 下仍应存在带可访问名称的抽屉按钮。
  it('768px 下保留可操作导航按钮', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 768 })
    const wrapper = await mountApp()
    const menu = wrapper.get('[aria-label="打开导航"]')
    expect(menu.attributes('aria-label')).toBe('打开导航')
    await menu.trigger('click')
    expect(wrapper.get('.app-sidebar').classes()).toContain('open')
  })

  // 测试参考图隔离：所有页面不得出现参考图中的演示来源和应用名称。
  it('页面中不存在参考图演示数据', async () => {
    const wrapper = await mountApp()
    for (const label of ['总览', '硬盘', '唤醒事件', '疑似来源', '诊断', '设置', '关于']) await navigate(wrapper, label)
    expect(wrapper.text()).not.toContain('Windows Update')
    expect(wrapper.text()).not.toContain('Chrome')
    expect(wrapper.text()).not.toContain('Steam')
  })

  // 测试事件分页：下一页操作必须追加 page=2 请求，而不是在浏览器载入全部历史。
  it('事件列表保持后端分页', async () => {
    const mockFetch = vi.fn((url: string | URL | Request, init?: RequestInit) => {
      const target = String(url)
      if (target.includes('/events')) {
        const page = Number(new URL(`http://local${target}`).searchParams.get('page') || 1)
        return response({ items: [wakeEvent], page, pageSize: 20, total: 25 })
      }
      return baseFetch(url, init)
    })
    vi.stubGlobal('fetch', mockFetch)
    const wrapper = await mountApp()
    await navigate(wrapper, '唤醒事件')
    await wrapper.findAll('button').find((button) => button.text().includes('下一页'))!.trigger('click')
    await flushPromises()
    expect(mockFetch.mock.calls.some(([url]) => String(url).includes('page=2'))).toBe(true)
  })
})
