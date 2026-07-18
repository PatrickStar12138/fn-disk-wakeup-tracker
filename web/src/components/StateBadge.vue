<script setup lang="ts">
import { stateLabel, type DiskState } from '../api'

defineProps<{ state: DiskState }>()
</script>

<template><span :class="['state-badge', state]"><span class="state-marker"></span>{{ stateLabel[state] }}</span></template>
