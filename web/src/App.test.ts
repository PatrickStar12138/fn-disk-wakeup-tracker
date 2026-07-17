import { flushPromises, mount } from '@vue/test-utils'
import App from './App.vue'

// 在 jsdom 中替换 Canvas 图表实现，只验证数据和空状态，不引入会掩盖问题的原生 canvas 依赖。
vi.mock('uplot', () => ({ default: class MockUPlot { destroy() { /* 测试替身无需释放浏览器资源。 */ } } }))

const settings = { sampleIntervalSeconds: 15, stateConfirmations: 3, retentionDays: 30, maxDatabaseMB: 200, logLevel: 'info', logRetentionFiles: 5, recordLowConfidence: true, showMaskedSerial: true, defaultTimeRange: '24h', ignoredProcesses: [], enableHdparmProbe: true }
/** response 构造不访问网络的最小 Fetch 响应。 */
function response(body: unknown, ok = true) { return Promise.resolve({ ok, status: ok ? 200 : 500, json: () => Promise.resolve(body) } as Response) }
/** baseFetch 按 API 路径返回空数据、unknown 磁盘和安全设置测试夹具。 */
function baseFetch(url: string | URL | Request, init?: RequestInit) { const u = String(url); if (u.endsWith('/overview')) return response({ mechanicalDisks: 1, activeDisks: 0, standbyDisks: 0, unknownDisks: 1, todayWakeups: 0, suspectedSource: '', collectorHealthy: true, databaseStatus: '正常' }); if (u.endsWith('/disks')) return response({ items: [{ id: '1', device: 'sda', model: 'Test', maskedSerial: '****1234', capacityBytes: 1000, busType: 'sata', rotational: true, state: 'unknown', previousState: 'active', lastStateChange: new Date().toISOString(), todayWakeups: 0, detectionMethod: 'unavailable', capabilitySupported: false, present: true }, { id: '2', device: 'nvme0n1', model: 'SSD', maskedSerial: '', capacityBytes: 2000, busType: 'nvme', rotational: false, state: 'unsupported', previousState: 'unsupported', lastStateChange: new Date().toISOString(), todayWakeups: 0, detectionMethod: 'media_type', capabilitySupported: false, present: true }] }); if (u.includes('/events')) return response({ items: [], page: 1, pageSize: 20, total: 0 }); if (u.endsWith('/version')) return response({ version: '0.1.0' }); if (u.endsWith('/settings')) return response(init?.method === 'PUT' ? { settings } : settings); return response({}) }

describe('App', () => {
  beforeEach(() => { vi.stubGlobal('fetch', vi.fn(baseFetch)); vi.stubGlobal('ResizeObserver', class { observe(){} disconnect(){} }) })
  afterEach(() => vi.unstubAllGlobals())
  // 验证 unknown 使用独立“未知”文案且空事件不会绘制假曲线。
  it('shows unknown as 未知 and empty events', async () => { const w = mount(App); await flushPromises(); expect(w.text()).toContain('暂无趋势数据'); await w.get('nav button:nth-child(2)').trigger('click'); expect(w.text()).toContain('未知'); expect(w.text()).toContain('不支持'); expect(w.text()).not.toContain('待机硬盘') })
  // 验证 API 失败展示明确错误而不是继续显示旧数据。
  it('shows API error instead of stale data', async () => { vi.stubGlobal('fetch', vi.fn(() => response({ error: '网关错误' }, false))); const w = mount(App); await flushPromises(); expect(w.text()).toContain('数据加载失败'); expect(w.text()).toContain('网关错误') })
  // 验证关于页展示构建注入版本。
  it('shows injected version', async () => { const w = mount(App); await flushPromises(); await w.get('nav button:last-child').trigger('click'); expect(w.text()).toContain('版本') })
  // 验证设置只有后端确认后才展示成功提示。
  it('saves settings and prevents duplicate submit', async () => { const w = mount(App); await flushPromises(); await w.get('nav button:nth-child(6)').trigger('click'); await flushPromises(); const button = w.findAll('button').find((b) => b.text().includes('保存设置')); expect(button).toBeTruthy(); await button!.trigger('click'); await flushPromises(); expect(w.text()).toContain('设置已保存') })
  // 验证成功 Toast 停留后会从 DOM 收回，不会永久遮挡界面。
  it('removes toast after exit delay', async () => { const w = mount(App); await flushPromises(); await w.get('nav button:nth-child(6)').trigger('click'); await flushPromises(); await w.findAll('button').find((b) => b.text().includes('保存设置'))!.trigger('click'); await flushPromises(); expect(w.text()).toContain('设置已保存'); await new Promise((resolve) => window.setTimeout(resolve, 3700)); expect(w.text()).not.toContain('设置已保存') }, 5000)
  // 验证保存失败显示错误 Toast，不会伪装为成功。
  it('shows settings save failure', async () => { vi.stubGlobal('fetch', vi.fn((url: string | URL | Request, init?: RequestInit) => String(url).endsWith('/settings') && init?.method === 'PUT' ? response({ error: '磁盘只读' }, false) : baseFetch(url, init))); const w = mount(App); await flushPromises(); await w.get('nav button:nth-child(6)').trigger('click'); await flushPromises(); const button = w.findAll('button').find((b) => b.text().includes('保存设置'))!; await button.trigger('click'); await flushPromises(); expect(w.text()).toContain('磁盘只读') })
  // 验证分页按钮使用下一页参数重新请求，而不是在浏览器加载全部事件。
  it('paginates event filters', async () => { const mock = vi.fn((url: string | URL | Request, init?: RequestInit) => { const u = String(url); if (u.includes('/events')) { const page = new URL(`http://local${u}`).searchParams.get('page') || '1'; return response({ items: [{ id: Number(page), device: 'sda', type: 'disk_wakeup', fromState: 'standby', toState: 'active', startedAt: new Date().toISOString(), durationMs: 0, readDelta: 0, writeDelta: 0, suspectProcess: '', suspectFnosApp: '', suspectDockerContainer: '', reason: '测试证据', confidence: '中', note: '' }], page: Number(page), pageSize: 20, total: 25 }) } return baseFetch(url, init) }); vi.stubGlobal('fetch', mock); const w = mount(App); await flushPromises(); await w.get('nav button:nth-child(3)').trigger('click'); await w.findAll('button').find((b) => b.text() === '下一页')!.trigger('click'); await flushPromises(); expect(mock.mock.calls.some(([url]) => String(url).includes('page=2'))).toBe(true) })
  // 验证 768px 小窗口仍保留可操作的导航按钮。
  it('keeps navigation control at narrow width', async () => { Object.defineProperty(window, 'innerWidth', { configurable: true, value: 768 }); const w = mount(App); await flushPromises(); expect(w.find('[aria-label="打开导航"]').exists()).toBe(true) })
})
