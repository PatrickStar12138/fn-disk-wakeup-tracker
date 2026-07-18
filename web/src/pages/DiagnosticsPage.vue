<script setup lang="ts">
import { computed } from 'vue'
import { API_BASE, formatBytes, formatTime, type DiagnosticReport } from '../api'
import EmptyState from '../components/EmptyState.vue'
import GlassPanel from '../components/GlassPanel.vue'
import LocalIcon from '../components/LocalIcon.vue'

const props = defineProps<{ report?: DiagnosticReport }>()
const emit = defineEmits<{ notify: [message: string, kind: 'success' | 'error'] }>()

/** reportJSON 将当前真实诊断对象格式化为可复制预览，缺失时保持空内容。 */
const reportJSON = computed(() => props.report ? JSON.stringify(props.report, null, 2) : '')

/** healthLabel 将诊断中的健康字符串转换为中文，并保留未知降级状态。 */
function healthLabel(value?: string): string {
  if (!value) return '未知'
  if (['healthy', 'running', 'online', 'ok', '正常'].includes(value.toLowerCase())) return '正常'
  return value
}

/** healthTone 根据真实健康值决定状态样式，未知值不会显示为成功。 */
function healthTone(value?: string): 'success' | 'warning' | 'danger' {
  if (!value) return 'warning'
  return ['healthy', 'running', 'online', 'ok', '正常'].includes(value.toLowerCase()) ? 'success' : 'danger'
}

/** copyReport 复制当前脱敏诊断 JSON，浏览器拒绝剪贴板权限时反馈明确错误。 */
async function copyReport(): Promise<void> {
  if (!reportJSON.value) return
  try {
    await navigator.clipboard.writeText(reportJSON.value)
    emit('notify', '诊断 JSON 已复制', 'success')
  } catch {
    emit('notify', '复制失败，请检查浏览器剪贴板权限', 'error')
  }
}
</script>

<template>
  <template v-if="report">
    <section class="diagnostic-cards" aria-label="诊断状态摘要">
      <article v-for="item in [
        { label: 'Server 状态', value: report.serverStatus, icon: 'server' },
        { label: 'Collector 状态', value: report.collectorStatus, icon: 'collector' },
        { label: '数据库状态', value: report.databaseStatus, icon: 'database' },
      ]" :key="item.label" class="diagnostic-card glass-surface">
        <span class="icon-disc"><LocalIcon :name="item.icon" /></span>
        <div><small>{{ item.label }}</small><strong><i :class="['status-dot', healthTone(item.value)]"></i>{{ healthLabel(item.value) }}</strong></div>
      </article>
      <article class="diagnostic-card glass-surface">
        <span class="icon-disc"><LocalIcon name="shield" /></span>
        <div><small>版本一致性</small><strong><i :class="['status-dot', report.versionConsistent === true ? 'success' : report.versionConsistent === false ? 'danger' : 'warning']"></i>{{ report.versionConsistent === true ? '一致' : report.versionConsistent === false ? '不一致' : '未知' }}</strong></div>
      </article>
    </section>

    <div class="report-actions">
      <a class="button secondary" :href="`${API_BASE}/diagnostics.txt`"><LocalIcon name="download" />下载文本报告</a>
      <a class="button secondary" :href="`${API_BASE}/diagnostics.json`"><LocalIcon name="code" />下载 JSON 报告</a>
    </div>

    <section class="diagnostic-layout">
      <GlassPanel class="report-preview">
        <div class="panel-heading"><div><small>脱敏报告</small><h2>诊断 JSON 预览</h2></div><button class="button secondary small" type="button" @click="copyReport"><LocalIcon name="copy" />复制</button></div>
        <pre>{{ reportJSON }}</pre>
      </GlassPanel>
      <GlassPanel class="report-details">
        <div class="panel-heading"><div><small>报告说明</small><h2>环境摘要</h2></div><LocalIcon name="shield" /></div>
        <p>报告会对序列号等敏感标识进行脱敏，不包含 Token、密码、Cookie、完整环境变量或任意文件内容。</p>
        <dl class="detail-list">
          <div><dt>生成时间</dt><dd>{{ formatTime(report.generatedAt) }}</dd></div>
          <div><dt>fnOS 版本</dt><dd>{{ report.fnosVersion || '未知' }}</dd></div>
          <div><dt>内核版本</dt><dd>{{ report.kernelVersion || '未知' }}</dd></div>
          <div><dt>CPU 架构</dt><dd>{{ report.architecture || '未知' }}</dd></div>
          <div><dt>数据库大小</dt><dd>{{ formatBytes(report.databaseSizeBytes || 0) }}</dd></div>
          <div><dt>Schema 版本</dt><dd>{{ report.schemaVersion ?? '未知' }}</dd></div>
          <div><dt>可用命令</dt><dd>{{ Object.entries(report.availableCommands || {}).filter(([, available]) => available).map(([name]) => name).join('、') || '无可用命令信息' }}</dd></div>
        </dl>
      </GlassPanel>
    </section>
  </template>
  <EmptyState v-else icon="diagnostics" title="暂无诊断报告" description="重新加载页面后将展示后端返回的真实诊断信息。" />
</template>
