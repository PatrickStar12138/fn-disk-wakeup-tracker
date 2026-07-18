<script setup lang="ts">
import { rangeLabel, type EventFilters, type SourceRow } from '../api'
import EmptyState from '../components/EmptyState.vue'
import GlassPanel from '../components/GlassPanel.vue'
import LocalIcon from '../components/LocalIcon.vue'
import MetricCard from '../components/MetricCard.vue'

defineProps<{ rows: SourceRow[]; filters: EventFilters }>()
</script>

<template>
  <div class="page-stack page-enter">
    <p class="page-context">基于当前事件筛选结果汇总</p>
    <section class="source-metrics metric-grid three">
      <MetricCard label="来源条目数" :value="rows.length" helper="当前事件页已识别条目" icon="sources" tone="blue" />
      <MetricCard label="高可信度条目数" :value="rows.filter((row) => row.confidence === '高').length" helper="仅统计当前事件页" icon="shield" tone="green" />
      <MetricCard label="当前统计范围" :value="rangeLabel[filters.range]" helper="受当前筛选条件约束" icon="clock" tone="violet" />
    </section>
    <GlassPanel title="来源汇总（按当前页出现次数）" icon="sources" :padded="false" class="table-panel source-panel">
      <div v-if="rows.length" class="table-scroll"><table class="data-table source-table"><thead><tr><th>序号</th><th>来源</th><th>来源类型</th><th>当前页出现次数</th><th>可信度</th><th>判断依据</th></tr></thead><tbody><tr v-for="(row, index) in rows" :key="row.source"><td>{{ index + 1 }}</td><td><strong class="source-name"><LocalIcon :name="row.type === 'Docker 容器' ? 'container' : row.type === 'fnOS 应用' ? 'app' : 'process'" :size="18" />{{ row.source }}</strong></td><td>{{ row.type }}</td><td><strong>{{ row.count }}</strong></td><td><span :class="['confidence-badge', `confidence-${row.confidence || '低'}`]">{{ row.confidence || '低' }}</span></td><td class="evidence-cell">{{ row.reason }}</td></tr></tbody></table></div>
      <EmptyState v-else title="暂无可归因证据" description="当前事件筛选结果中没有可汇总的来源字段" icon="sources" />
    </GlassPanel>
    <GlassPanel class="correlation-note"><div class="notice-row"><span><LocalIcon name="info" :size="20" /></span><div><strong>关于疑似来源</strong><p>以上结果仅基于当前事件页的时间相关性分析，不是完整历史排行，也不代表确定因果关系。本应用自身 I/O 默认不作为疑似来源。</p></div></div></GlassPanel>
  </div>
</template>
