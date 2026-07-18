<script setup lang="ts">
import { ref, watch } from 'vue'
import type { Settings } from '../api'
import GlassPanel from '../components/GlassPanel.vue'
import LocalIcon from '../components/LocalIcon.vue'

const props = defineProps<{ settings: Settings; saving: boolean }>()
const emit = defineEmits<{ save: [settings: Settings] }>()
const draft = ref<Settings>(cloneSettings(props.settings))
const ignoredProcesses = ref(props.settings.ignoredProcesses.join('\n'))

/** cloneSettings 深拷贝后端已确认设置，避免编辑草稿污染最后一次成功值。 */
function cloneSettings(value: Settings): Settings {
  return { ...value, ignoredProcesses: [...value.ignoredProcesses] }
}

/** syncConfirmedSettings 在后端成功返回后同步新确认值，不响应本地草稿变化。 */
function syncConfirmedSettings(value: Settings): void {
  draft.value = cloneSettings(value)
  ignoredProcesses.value = value.ignoredProcesses.join('\n')
}

/** restoreCurrent 将表单恢复到最近一次后端成功返回的值，而不是猜测默认设置。 */
function restoreCurrent(): void {
  syncConfirmedSettings(props.settings)
}

/** submitSettings 整理逐行精确进程名后交由父组件保存，后端仍会执行范围校验。 */
function submitSettings(): void {
  if (props.saving) return
  emit('save', {
    ...cloneSettings(draft.value),
    ignoredProcesses: ignoredProcesses.value.split('\n').map((name) => name.trim()).filter(Boolean),
  })
}

watch(() => props.settings, syncConfirmedSettings, { deep: true })
</script>

<template>
  <form class="settings-form" @submit.prevent="submitSettings">
    <section class="settings-columns">
      <GlassPanel>
        <div class="panel-heading"><div><small>低干扰采集</small><h2>采集与数据库</h2></div><LocalIcon name="database" /></div>
        <div class="form-stack">
          <label><span><strong>采样间隔（秒）</strong><small>数据采集时间间隔，允许 5–300 秒</small></span><input v-model.number="draft.sampleIntervalSeconds" type="number" min="5" max="300" required /></label>
          <label><span><strong>状态确认次数</strong><small>连续确认后才记录稳定状态，允许 1–10 次</small></span><input v-model.number="draft.stateConfirmations" type="number" min="1" max="10" required /></label>
          <label><span><strong>保留天数</strong><small>历史事件保留期限，允许 1–365 天</small></span><input v-model.number="draft.retentionDays" type="number" min="1" max="365" required /></label>
          <label><span><strong>数据库上限（MB）</strong><small>数据库体积保护上限，允许 20–2048 MB</small></span><input v-model.number="draft.maxDatabaseMB" type="number" min="20" max="2048" required /></label>
          <label><span><strong>默认时间范围</strong><small>打开事件页时采用的筛选范围</small></span><select v-model="draft.defaultTimeRange"><option value="24h">近 24 小时</option><option value="7d">近 7 天</option><option value="30d">近 30 天</option></select></label>
        </div>
      </GlassPanel>
      <GlassPanel>
        <div class="panel-heading"><div><small>输出与能力</small><h2>日志与高级</h2></div><LocalIcon name="settings" /></div>
        <div class="form-stack">
          <label><span><strong>日志级别</strong><small>控制有限滚动日志的详细程度</small></span><select v-model="draft.logLevel"><option value="error">错误</option><option value="warn">警告</option><option value="info">信息</option><option value="debug">调试</option></select></label>
          <label><span><strong>日志保留数量</strong><small>滚动日志文件数量，允许 1–20 个</small></span><input v-model.number="draft.logRetentionFiles" type="number" min="1" max="20" required /></label>
          <label class="switch-row"><span><strong>记录低可信度来源</strong><small>保留证据较弱的时间相关性线索</small></span><input v-model="draft.recordLowConfidence" class="switch" type="checkbox" /></label>
          <label class="switch-row"><span><strong>显示脱敏序列号</strong><small>仅展示后端返回的脱敏值</small></span><input v-model="draft.showMaskedSerial" class="switch" type="checkbox" /></label>
          <label class="switch-row"><span><strong>启用 hdparm -C 探测 <em>实验性</em></strong><small>不同控制器行为需 fnOS 真机验证，不支持时仍显示未知</small></span><input v-model="draft.enableHdparmProbe" class="switch" type="checkbox" /></label>
          <label class="textarea-row"><span><strong>忽略进程列表</strong><small>每行一个精确进程名称，当前不支持通配符</small></span><textarea v-model="ignoredProcesses" rows="5" maxlength="12900" /></label>
        </div>
      </GlassPanel>
    </section>
    <div class="settings-actions glass-surface">
      <button class="button secondary" type="button" :disabled="saving" @click="restoreCurrent"><LocalIcon name="restore" />恢复当前值</button>
      <button class="button primary" type="submit" :disabled="saving"><LocalIcon :name="saving ? 'refresh' : 'save'" :class="{ spin: saving }" />{{ saving ? '保存中…' : '保存设置' }}</button>
    </div>
  </form>
</template>
