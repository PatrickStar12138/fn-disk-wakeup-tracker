<script setup lang="ts">
import { formatTime } from '../api'
import LocalIcon from './LocalIcon.vue'

defineProps<{ title: string; subtitle?: string; updatedAt?: string; refreshing: boolean }>()
defineEmits<{ refresh: []; menu: [] }>()
</script>

<template>
  <header class="app-header">
    <div class="header-title"><button class="icon-button menu-button" aria-label="打开导航" @click="$emit('menu')"><LocalIcon name="menu" /></button><div><h1>{{ title }}</h1><p v-if="subtitle">{{ subtitle }}</p></div></div>
    <div class="header-actions"><span class="updated">最后更新 {{ formatTime(updatedAt) }}</span><button class="button secondary" :disabled="refreshing" @click="$emit('refresh')"><LocalIcon name="refresh" :class="{ spin: refreshing }" :size="18" />{{ refreshing ? '刷新中' : '刷新' }}</button></div>
  </header>
</template>
