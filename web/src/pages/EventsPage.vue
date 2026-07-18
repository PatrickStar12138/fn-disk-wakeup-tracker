<script setup lang="ts">
import { eventSource, eventTypeLabel, formatBytes, formatTime, type Disk, type EventFilters, type EventRange, type WakeEvent } from '../api'
import EmptyState from '../components/EmptyState.vue'
import GlassPanel from '../components/GlassPanel.vue'
import LocalIcon from '../components/LocalIcon.vue'
import StateBadge from '../components/StateBadge.vue'

const props = defineProps<{ events: WakeEvent[]; disks: Disk[]; filters: EventFilters; page: number; pageSize: number; total: number; loading: boolean; exportUrl: string }>()
const emit = defineEmits<{ filters: [filters: EventFilters]; reload: []; page: [delta: number] }>()
const ranges: EventRange[] = ['24h', '7d', '30d']

/** updateFilter 创建新的筛选对象并回到第一页，避免直接修改父组件状态。 */
function updateFilter<K extends keyof EventFilters>(key: K, value: EventFilters[K]): void {
  emit('filters', { ...props.filters, [key]: value })
}

/** submitSource 在用户确认关键词时应用来源筛选，避免每次按键都请求后端。 */
function submitSource(event: Event): void {
  updateFilter('source', (event.target as HTMLInputElement).value)
}
</script>

<template>
  <div class="page-stack page-enter">
    <GlassPanel class="filter-panel">
      <div class="filter-layout">
        <div class="segmented" aria-label="时间范围"><button v-for="range in ranges" :key="range" :class="{ active: filters.range === range }" @click="updateFilter('range', range)">{{ range }}</button></div>
        <label><span>硬盘</span><select :value="filters.diskId" @change="updateFilter('diskId', ($event.target as HTMLSelectElement).value)"><option value="">全部</option><option v-for="disk in disks" :key="disk.id" :value="disk.id">/dev/{{ disk.device }}</option></select></label>
        <label><span>事件类型</span><select data-test="event-type-filter" :value="filters.type" @change="updateFilter('type', ($event.target as HTMLSelectElement).value)"><option value="">全部</option><option value="disk_wakeup">硬盘唤醒</option><option value="disk_standby">进入待机</option><option value="disk_activity">状态活动</option><option value="state_unknown">状态未知</option><option value="collector_offline">Collector 离线</option><option value="collector_recovered">Collector 恢复</option></select></label>
        <label><span>可信度</span><select :value="filters.confidence" @change="updateFilter('confidence', ($event.target as HTMLSelectElement).value as EventFilters['confidence'])"><option value="">全部</option><option value="高">高</option><option value="中">中</option><option value="低">低</option></select></label>
        <label class="source-filter"><span>疑似来源</span><input :value="filters.source" placeholder="输入关键词后回车" @keyup.enter="submitSource" /></label>
        <div class="filter-actions"><a class="button primary" :href="exportUrl" download><LocalIcon name="download" :size="17" />导出 CSV</a><button class="icon-button bordered" aria-label="重新加载事件" :disabled="loading" @click="$emit('reload')"><LocalIcon name="refresh" :class="{ spin: loading }" :size="18" /></button></div>
      </div>
    </GlassPanel>

    <GlassPanel :padded="false" class="table-panel events-panel">
      <div v-if="events.length" class="table-scroll"><table class="data-table events-table"><thead><tr><th>时间</th><th>硬盘</th><th>事件类型</th><th>原状态</th><th>新状态</th><th>读取增量</th><th>写入增量</th><th>疑似来源</th><th>判断依据</th><th>可信度</th></tr></thead><tbody><tr v-for="item in events" :key="item.id"><td>{{ formatTime(item.startedAt) }}</td><td><strong>{{ item.device ? `/dev/${item.device}` : '服务事件' }}</strong></td><td>{{ eventTypeLabel[item.type] || item.type }}</td><td><StateBadge :state="item.fromState" /></td><td><StateBadge :state="item.toState" /></td><td>{{ formatBytes(item.readDelta) }}</td><td>{{ formatBytes(item.writeDelta) }}</td><td>{{ eventSource(item) || '暂无证据' }}</td><td class="evidence-cell">{{ item.reason || '未形成有效判断依据' }}</td><td><span :class="['confidence-badge', `confidence-${item.confidence || '低'}`]">{{ item.confidence || '低' }}</span></td></tr></tbody></table></div>
      <EmptyState v-else title="筛选范围内没有事件" description="不会生成模拟唤醒事件，也不会把未知状态当成待机" icon="file" />
    </GlassPanel>
    <div class="pagination-bar"><p><LocalIcon name="info" :size="16" />事件列表由后端分页，CSV 导出沿用当前全部筛选条件。</p><div><button class="button secondary" :disabled="page <= 1 || loading" @click="$emit('page', -1)"><LocalIcon name="arrow-left" :size="15" />上一页</button><span>第 {{ page }} / {{ Math.max(1, Math.ceil(total / pageSize)) }} 页</span><button class="button secondary" :disabled="page * pageSize >= total || loading" @click="$emit('page', 1)">下一页<LocalIcon name="arrow-right" :size="15" /></button></div></div>
  </div>
</template>
