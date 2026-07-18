<script setup lang="ts">
import type { NavigationItem, Page } from '../api'
import LocalIcon from './LocalIcon.vue'

defineProps<{ items: NavigationItem[]; current: Page; open: boolean; iconUrl: string; collectorHealthy?: boolean; version: string }>()
defineEmits<{ navigate: [page: Page]; close: [] }>()
</script>

<template>
  <aside :class="['app-sidebar', { open }]">
    <div class="sidebar-brand"><img :src="iconUrl" alt="" /><div><strong>硬盘唤醒追踪器</strong><small>Disk observability</small></div></div>
    <nav aria-label="主导航"><button v-for="item in items" :key="item.id" :class="{ active: current === item.id }" @click="$emit('navigate', item.id)"><LocalIcon :name="item.icon" :size="20" /><span>{{ item.label }}</span></button></nav>
    <div class="sidebar-status"><span :class="['status-dot', collectorHealthy ? 'ok' : 'bad']"></span><strong>{{ collectorHealthy ? '采集服务正常' : '采集服务离线' }}</strong><small>v{{ version }}</small></div>
  </aside>
  <button v-if="open" class="sidebar-scrim" aria-label="关闭导航" @click="$emit('close')"></button>
</template>
