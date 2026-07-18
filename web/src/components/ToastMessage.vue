<script setup lang="ts">
import LocalIcon from './LocalIcon.vue'

defineProps<{ visible: boolean; message: string; kind: 'success' | 'error' }>()
</script>

<template>
  <Transition name="toast-slide"><div v-if="visible" :class="['toast', kind]" :role="kind === 'error' ? 'alert' : 'status'"><LocalIcon :name="kind === 'success' ? 'check' : 'warning'" :size="19" /><span>{{ message }}</span></div></Transition>
</template>
