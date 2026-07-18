<script setup lang="ts">
import { computed } from 'vue'
import LocalIcon from './LocalIcon.vue'

/** StatusTone 是全局状态项允许使用的语义色，不以颜色替代文字。 */
type StatusTone = 'success' | 'warning' | 'danger' | 'unknown'

/** StatusPresentation 描述单个全局状态项的完整、紧凑文案与语义色。 */
interface StatusPresentation {
  /** label 是宽窗口使用的完整可读文案。 */
  label: string
  /** shortLabel 是 1024px 及以下使用的紧凑文案。 */
  shortLabel: string
  /** tone 是状态点和图标采用的语义色。 */
  tone: StatusTone
}

const props = defineProps<{ collectorHealthy?: boolean; databaseStatus?: string; lastRefreshAt?: string }>()

/** collectorStatus 严格区分正常、离线和未知，未取得总览数据时不会默认显示绿色。 */
const collectorStatus = computed<StatusPresentation>(() => {
  if (props.collectorHealthy === true) return { label: '采集服务正常', shortLabel: '采集正常', tone: 'success' }
  if (props.collectorHealthy === false) return { label: '采集服务离线', shortLabel: '采集离线', tone: 'danger' }
  return { label: '采集状态未知', shortLabel: '采集未知', tone: 'unknown' }
})

/** receptionStatus 根据真实 Collector 状态和最近采集时间展示接收结果，不把 API 请求成功当作采集成功。 */
const receptionStatus = computed<StatusPresentation>(() => {
  if (props.collectorHealthy === undefined) return { label: '数据状态未知', shortLabel: '数据未知', tone: 'unknown' }
  if (!props.lastRefreshAt) return { label: '等待首次采集', shortLabel: '等待采集', tone: 'warning' }
  if (props.collectorHealthy === false) return { label: '采集数据已中断', shortLabel: '数据中断', tone: 'warning' }
  return { label: '已接收采集数据', shortLabel: '数据已接收', tone: 'success' }
})

/** databaseHealthStatus 识别后端已有健康文案，其余非空值均保守显示异常，空值显示未知。 */
const databaseHealthStatus = computed<StatusPresentation>(() => {
  const normalized = props.databaseStatus?.trim().toLowerCase()
  if (!normalized) return { label: '数据库状态未知', shortLabel: '数据库未知', tone: 'unknown' }
  if (['正常', 'healthy', 'ok'].includes(normalized)) return { label: '数据库正常', shortLabel: '数据库正常', tone: 'success' }
  return { label: '数据库异常', shortLabel: '数据库异常', tone: 'danger' }
})
</script>

<template>
  <footer class="global-status-bar" aria-label="全局运行状态" aria-live="polite">
    <div class="global-status-items">
      <span :class="['global-status-item', `tone-${collectorStatus.tone}`]" data-status-item="collector">
        <i class="status-dot" aria-hidden="true"></i><span class="status-label-full">{{ collectorStatus.label }}</span><span class="status-label-short">{{ collectorStatus.shortLabel }}</span>
      </span>
      <span class="status-separator" aria-hidden="true"></span>
      <span :class="['global-status-item', `tone-${receptionStatus.tone}`]" data-status-item="reception">
        <LocalIcon name="collector" :size="15" /><span class="status-label-full">{{ receptionStatus.label }}</span><span class="status-label-short">{{ receptionStatus.shortLabel }}</span>
      </span>
      <span class="status-separator" aria-hidden="true"></span>
      <span :class="['global-status-item', `tone-${databaseHealthStatus.tone}`]" data-status-item="database">
        <LocalIcon name="database" :size="15" /><span class="status-label-full">{{ databaseHealthStatus.label }}</span><span class="status-label-short">{{ databaseHealthStatus.shortLabel }}</span>
      </span>
    </div>
  </footer>
</template>
