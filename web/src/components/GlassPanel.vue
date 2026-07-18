<script setup lang="ts">
import LocalIcon from './LocalIcon.vue'

withDefaults(defineProps<{ title?: string; eyebrow?: string; icon?: string; padded?: boolean }>(), { title: '', eyebrow: '', icon: '', padded: true })
</script>

<template>
  <section :class="['glass-panel', { 'no-padding': !padded }]">
    <header v-if="title || eyebrow || icon || $slots.action" class="panel-heading">
      <div class="panel-heading-main">
        <span v-if="icon" class="panel-icon"><LocalIcon :name="icon" :size="19" /></span>
        <div><span v-if="eyebrow" class="eyebrow">{{ eyebrow }}</span><h2 v-if="title">{{ title }}</h2></div>
      </div>
      <div v-if="$slots.action" class="panel-action"><slot name="action" /></div>
    </header>
    <slot />
  </section>
</template>
