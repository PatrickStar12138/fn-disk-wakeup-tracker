<script setup lang="ts">
import { eventSource, eventTypeLabel, formatTime, type Disk, type Overview, type Page, type WakeEvent } from '../api'
import EmptyState from '../components/EmptyState.vue'
import GlassPanel from '../components/GlassPanel.vue'
import LocalIcon from '../components/LocalIcon.vue'
import MetricCard from '../components/MetricCard.vue'
import StateBadge from '../components/StateBadge.vue'
import WakeChart from '../components/WakeChart.vue'

defineProps<{ overview?: Overview; disks: Disk[]; events: WakeEvent[] }>()
defineEmits<{ navigate: [page: Page] }>()
</script>

<template>
  <div class="page-stack page-enter">
    <section class="metric-grid">
      <MetricCard label="机械硬盘数量" :value="overview?.mechanicalDisks ?? 0" helper="已识别物理盘" icon="disk" tone="blue" />
      <MetricCard label="当前活动数量" :value="overview?.activeDisks ?? 0" helper="活动或空闲" icon="collector" tone="green" />
      <MetricCard label="当前待机数量" :value="overview?.standbyDisks ?? 0" helper="连续确认后计入" icon="clock" tone="violet" />
      <MetricCard label="今日唤醒次数" :value="overview?.todayWakeups ?? 0" :helper="overview?.lastWakeupAt ? `最近 ${formatTime(overview.lastWakeupAt)}` : '今日暂无唤醒事件'" icon="bell" tone="orange" />
    </section>

    <section class="overview-grid">
      <GlassPanel title="硬盘状态" icon="disk">
        <template #action><button class="text-button" @click="$emit('navigate', 'disks')">查看全部 <LocalIcon name="arrow-right" :size="15" /></button></template>
        <div v-if="disks.length" class="compact-table disk-summary">
          <div class="compact-row compact-head"><span>设备名</span><span>型号</span><span>当前状态</span><span>今日唤醒</span></div>
          <div v-for="disk in disks.slice(0, 5)" :key="disk.id" class="compact-row">
            <strong><i :class="['device-dot', disk.state]"></i>/dev/{{ disk.device }}</strong><span>{{ disk.model || '未知型号' }}</span><StateBadge :state="disk.state" /><span>{{ disk.todayWakeups }} 次</span>
          </div>
        </div>
        <EmptyState v-else title="暂无硬盘数据" description="等待 Collector 完成首次安全扫描" icon="disk" compact />
      </GlassPanel>

      <GlassPanel title="近 24 小时唤醒趋势" icon="clock"><template #action><span class="panel-stat">共 {{ events.filter((event) => event.type === 'disk_wakeup').length }} 条当前页事件</span></template><WakeChart :events="events" /></GlassPanel>

      <GlassPanel title="最近事件" icon="events">
        <template #action><button class="text-button" @click="$emit('navigate', 'events')">查看全部 <LocalIcon name="arrow-right" :size="15" /></button></template>
        <div v-if="events.length" class="event-summary-list"><article v-for="event in events.slice(0, 5)" :key="event.id"><span class="summary-icon"><LocalIcon name="events" :size="17" /></span><div><strong>{{ eventTypeLabel[event.type] || event.type }}</strong><small>{{ event.device ? `/dev/${event.device}` : '服务事件' }} · {{ formatTime(event.startedAt) }}</small></div><span class="summary-source">{{ eventSource(event) || '暂无疑似来源' }}</span></article></div>
        <EmptyState v-else title="暂无唤醒事件" description="当前筛选范围内没有真实事件记录" icon="file" compact />
      </GlassPanel>

      <GlassPanel title="疑似来源摘要" icon="sources">
        <template #action><button class="text-button" @click="$emit('navigate', 'sources')">查看全部 <LocalIcon name="arrow-right" :size="15" /></button></template>
        <div v-if="events.some((event) => eventSource(event))" class="source-summary"><span class="source-orb"><LocalIcon name="sources" :size="25" /></span><div><small>当前事件页最近证据</small><strong>{{ events.map(eventSource).find(Boolean) }}</strong><p>仅表示状态变化前后的时间相关性，不代表确定因果关系。</p></div></div>
        <EmptyState v-else title="暂无可归因证据" description="当前事件页没有可用的疑似来源字段" icon="sources" compact />
      </GlassPanel>
    </section>

    <GlassPanel class="capability-note" :padded="true"><div class="notice-row"><span><LocalIcon name="info" :size="20" /></span><div><strong>能力限制</strong><p>未知硬盘 {{ overview?.unknownDisks ?? 0 }} 块。“未知”仅表示无法可靠获取电源状态；SSD/NVMe 不适用机械盘待机状态。</p></div></div></GlassPanel>
  </div>
</template>
