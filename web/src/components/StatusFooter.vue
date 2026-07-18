<script setup lang="ts">
import LocalIcon from './LocalIcon.vue'

defineProps<{ collectorHealthy?: boolean; databaseStatus?: string; lastRefreshAt?: string }>()
</script>

<template>
  <footer class="status-footer" aria-label="应用状态">
    <span><i :class="['status-dot', collectorHealthy ? 'ok' : 'bad']"></i>{{ collectorHealthy ? '采集服务正常' : '采集服务离线' }}</span>
    <span class="footer-divider"></span><span><LocalIcon name="collector" :size="15" />{{ lastRefreshAt ? '已接收采集数据' : '等待首次上报' }}</span>
    <span class="footer-divider"></span><span><LocalIcon name="database" :size="15" />数据库 {{ databaseStatus || '未知' }}</span>
  </footer>
</template>
