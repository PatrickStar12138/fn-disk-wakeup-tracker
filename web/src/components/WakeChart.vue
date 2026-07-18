<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import type { WakeEvent } from '../api'
import EmptyState from './EmptyState.vue'

const props = defineProps<{ events: WakeEvent[] }>()
const host = ref<HTMLElement>()
let chart: uPlot | undefined
let observer: ResizeObserver | undefined

/** isWakeEvent 只接受后端明确标记的硬盘唤醒事件，服务状态事件不会生成零值假曲线。 */
function isWakeEvent(event: WakeEvent): boolean {
  return event.type === 'disk_wakeup' && Number.isFinite(new Date(event.startedAt).getTime())
}

/** wakeEvents 提取当前事件页中的真实唤醒记录，空集合直接展示空状态。 */
const wakeEvents = computed(() => props.events.filter(isWakeEvent))

/** chartData 将真实唤醒事件按最近 24 个整点聚合，不生成随机或演示数据。 */
function chartData(): uPlot.AlignedData {
  const now = new Date()
  now.setMinutes(0, 0, 0)
  const hours = Array.from({ length: 24 }, (_, index) => new Date(now.getTime() - (23 - index) * 3_600_000))
  const counts = hours.map((hour) => wakeEvents.value.filter((event) => {
    const timestamp = new Date(event.startedAt).getTime()
    return timestamp >= hour.getTime() && timestamp < hour.getTime() + 3_600_000
  }).length)
  return [hours.map((value) => value.getTime() / 1000), counts]
}

/** renderChart 仅在存在真实唤醒事件时创建本地 uPlot 图表，并适配容器宽度。 */
function renderChart(): void {
  if (!host.value || wakeEvents.value.length === 0) return
  chart?.destroy()
  chart = new uPlot({
    width: host.value.clientWidth || 500,
    height: 210,
    cursor: { show: true },
    legend: { show: false },
    scales: { x: { time: true }, y: { range: [0, null] } },
    axes: [
      { stroke: '#78869a', grid: { stroke: 'rgba(112, 129, 154, .12)' } },
      { stroke: '#78869a', grid: { stroke: 'rgba(112, 129, 154, .12)' }, values: (_plot, ticks) => ticks.map((value) => String(Math.round(value))) },
    ],
    series: [{}, { label: '唤醒次数', stroke: '#149bb3', fill: 'rgba(20, 155, 179, .10)', width: 2, points: { show: false } }],
  }, chartData(), host.value)
}

/** handleEventsChanged 在真实事件变化后等待 DOM 更新并重绘图表。 */
function handleEventsChanged(): void {
  void nextTick(renderChart)
}

/** mountChart 创建尺寸观察器，使 iframe 宽度变化时图表保持在面板内。 */
function mountChart(): void {
  renderChart()
  observer = new ResizeObserver(renderChart)
  if (host.value) observer.observe(host.value)
}

/** unmountChart 释放图表和尺寸观察器，避免页面切换后残留监听。 */
function unmountChart(): void {
  observer?.disconnect()
  chart?.destroy()
}

watch(() => props.events, handleEventsChanged, { deep: true })
onMounted(mountChart)
onBeforeUnmount(unmountChart)
</script>

<template>
  <EmptyState v-if="wakeEvents.length === 0" compact icon="chart" title="暂无趋势数据" description="产生真实唤醒事件后将在这里显示曲线。" />
  <div v-else ref="host" class="chart-host" aria-label="最近 24 小时真实唤醒趋势图"></div>
</template>
