<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import type { WakeEvent } from '../api'

const props = defineProps<{ events: WakeEvent[] }>()
const host = ref<HTMLElement>()
let chart: uPlot | undefined
let observer: ResizeObserver | undefined

/** data 将真实唤醒事件按最近 24 个整点聚合，不生成模拟数据。 */
function data(): uPlot.AlignedData {
  const now = new Date(); now.setMinutes(0, 0, 0)
  const hours = Array.from({ length: 24 }, (_, i) => new Date(now.getTime() - (23 - i) * 3600_000))
  const counts = hours.map((hour) => props.events.filter((e) => e.type === 'disk_wakeup' && new Date(e.startedAt).getTime() >= hour.getTime() && new Date(e.startedAt).getTime() < hour.getTime() + 3600_000).length)
  return [hours.map((v) => v.getTime() / 1000), counts]
}
/** render 仅在存在真实事件时创建本地 uPlot 图表，并随容器宽度重绘。 */
function render() {
  if (!host.value || props.events.length === 0) return
  chart?.destroy()
  chart = new uPlot({ width: host.value.clientWidth || 500, height: 210, cursor: { show: true }, legend: { show: false }, scales: { x: { time: true }, y: { range: [0, null] } }, axes: [{ stroke: '#7b8794', grid: { stroke: '#d8dee722' } }, { stroke: '#7b8794', grid: { stroke: '#d8dee744' }, values: (_u, ticks) => ticks.map((v) => String(Math.round(v))) }], series: [{}, { label: '唤醒次数', stroke: '#20a6b7', fill: '#20a6b718', width: 2, points: { show: false } }] }, data(), host.value)
}
watch(() => props.events, () => nextTick(render), { deep: true })
onMounted(() => { render(); observer = new ResizeObserver(render); if (host.value) observer.observe(host.value) })
onBeforeUnmount(() => { observer?.disconnect(); chart?.destroy() })
</script>
<template>
  <div v-if="events.length === 0" class="empty compact"><span class="empty-icon">⌁</span><strong>暂无趋势数据</strong><small>产生唤醒事件后将在这里显示真实曲线</small></div>
  <div v-else ref="host" class="chart-host" aria-label="最近 24 小时唤醒趋势图"></div>
</template>
