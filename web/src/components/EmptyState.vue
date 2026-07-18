<script setup lang="ts">
import LocalIcon from './LocalIcon.vue'

withDefaults(defineProps<{ title: string; description?: string; icon?: string; compact?: boolean }>(), { description: '', icon: 'file', compact: false })
</script>

<template>
  <div :class="['empty-state', { compact }]">
    <span class="empty-illustration"><LocalIcon :name="icon" :size="32" /></span>
    <strong>{{ title }}</strong><p v-if="description">{{ description }}</p><slot />
  </div>
</template>
