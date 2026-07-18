<script setup lang="ts">
import LocalIcon from './LocalIcon.vue'

withDefaults(defineProps<{ message: string; title?: string }>(), { title: '数据加载失败' })
defineEmits<{ retry: [] }>()
</script>

<template>
  <section class="error-state" role="alert"><span><LocalIcon name="warning" :size="26" /></span><strong>{{ title }}</strong><p>{{ message }}</p><button class="button secondary" @click="$emit('retry')">重试</button></section>
</template>
