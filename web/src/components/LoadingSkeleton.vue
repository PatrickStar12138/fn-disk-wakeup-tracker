<script setup lang="ts">
withDefaults(defineProps<{ count?: number }>(), { count: 8 })
</script>

<template><section class="skeleton-grid" aria-label="正在加载" aria-live="polite"><span class="sr-only">正在加载页面数据</span><div v-for="item in count" :key="item" class="skeleton"></div></section></template>
