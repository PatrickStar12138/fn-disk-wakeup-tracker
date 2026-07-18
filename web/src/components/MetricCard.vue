<script setup lang="ts">
import LocalIcon from './LocalIcon.vue'

withDefaults(defineProps<{ label: string; value: string | number; helper?: string; icon: string; tone?: string }>(), { helper: '', tone: 'blue' })
</script>

<template>
  <article class="metric-card glass-panel">
    <span :class="['metric-icon', `tone-${tone}`]"><LocalIcon :name="icon" :size="22" /></span>
    <div class="metric-copy"><span>{{ label }}</span><strong>{{ value }}</strong><small>{{ helper || '—' }}</small></div>
  </article>
</template>
