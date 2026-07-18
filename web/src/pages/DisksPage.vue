<script setup lang="ts">
import { capabilityLabel, formatBytes, formatTime, type Disk } from '../api'
import EmptyState from '../components/EmptyState.vue'
import GlassPanel from '../components/GlassPanel.vue'
import LocalIcon from '../components/LocalIcon.vue'
import StateBadge from '../components/StateBadge.vue'

defineProps<{ disks: Disk[] }>()
</script>

<template>
  <div class="page-stack page-enter">
    <GlassPanel class="information-banner"><div class="notice-row"><span><LocalIcon name="info" :size="20" /></span><p><strong>“未知”状态并不代表硬盘处于休眠</strong>，仅表示当前无法可靠获取电源状态。SSD/NVMe 等非机械介质显示为“不支持”。</p></div></GlassPanel>
    <GlassPanel :padded="false" class="table-panel">
      <div v-if="disks.length" class="table-scroll"><table class="data-table disk-table"><thead><tr><th>设备名</th><th>介质类型</th><th>型号</th><th>脱敏序列号</th><th>容量</th><th>总线</th><th>当前状态</th><th>上一个状态</th><th>最后变化时间</th><th>今日唤醒</th><th>探测能力 / 检测方式</th></tr></thead><tbody><tr v-for="disk in disks" :key="disk.id"><td><strong class="device-cell"><i :class="['device-dot', disk.state]"></i>/dev/{{ disk.device }}</strong></td><td>{{ disk.rotational ? '机械硬盘' : '非机械介质' }}</td><td>{{ disk.model || '未知型号' }}</td><td>{{ disk.maskedSerial || '—' }}</td><td>{{ formatBytes(disk.capacityBytes) }}</td><td>{{ disk.busType || '—' }}</td><td><StateBadge :state="disk.state" /></td><td><StateBadge :state="disk.previousState" /></td><td>{{ formatTime(disk.lastStateChange) }}</td><td>{{ disk.todayWakeups }} 次</td><td><span class="capability" :title="disk.detectionMethod || '未提供检测方式'">{{ capabilityLabel(disk) }}<LocalIcon name="info" :size="14" /></span></td></tr></tbody></table></div>
      <EmptyState v-else title="暂无设备数据" description="页面不会通过读取磁盘文件强制获取状态" icon="disk" />
    </GlassPanel>
  </div>
</template>
