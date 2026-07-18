<script setup lang="ts">
withDefaults(defineProps<{ name: string; size?: number; strokeWidth?: number }>(), { size: 20, strokeWidth: 1.8 })
</script>

<template>
  <svg class="local-icon" :width="size" :height="size" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <g stroke="currentColor" :stroke-width="strokeWidth" stroke-linecap="round" stroke-linejoin="round">
      <template v-if="name === 'overview' || name === 'home'"><path d="M3.5 10.5 12 3.7l8.5 6.8"/><path d="M5.5 9.5v10h13v-10M9 19.5v-6h6v6"/></template>
      <template v-else-if="name === 'disk'"><circle cx="12" cy="12" r="8.5"/><circle cx="12" cy="12" r="2"/><path d="M12 5.5a6.5 6.5 0 0 1 6.5 6.5"/></template>
      <template v-else-if="name === 'events'"><path d="M13 2.8 6.7 13h5.2l-.9 8.2L17.3 11h-5.2l.9-8.2Z"/></template>
      <template v-else-if="name === 'sources'"><circle cx="12" cy="12" r="8.5"/><circle cx="12" cy="12" r="4.7"/><circle cx="12" cy="12" r="1.3"/></template>
      <template v-else-if="name === 'diagnostics'"><path d="M12 3v18M3 12h18"/><path d="M6 6h12v12H6z" opacity=".22"/></template>
      <template v-else-if="name === 'settings'"><path d="M4 7h10M18 7h2M4 17h2M10 17h10M14 4v6M7 14v6"/><circle cx="14" cy="7" r="1.8"/><circle cx="7" cy="17" r="1.8"/></template>
      <template v-else-if="name === 'about' || name === 'info'"><circle cx="12" cy="12" r="8.5"/><path d="M12 10.8v5.5M12 7.7h.01"/></template>
      <template v-else-if="name === 'refresh'"><path d="M19.5 8A8 8 0 1 0 20 14"/><path d="M19.5 3.8V8h-4.2"/></template>
      <template v-else-if="name === 'menu'"><path d="M4 7h16M4 12h16M4 17h16"/></template>
      <template v-else-if="name === 'close'"><path d="m6 6 12 12M18 6 6 18"/></template>
      <template v-else-if="name === 'server'"><rect x="4" y="4" width="16" height="6" rx="2"/><rect x="4" y="14" width="16" height="6" rx="2"/><path d="M8 7h.01M8 17h.01M12 7h5M12 17h5"/></template>
      <template v-else-if="name === 'collector' || name === 'wifi'"><path d="M4.5 9.5a11 11 0 0 1 15 0M7.5 13a6.5 6.5 0 0 1 9 0M10.5 16.5a2.3 2.3 0 0 1 3 0"/><circle cx="12" cy="19" r=".6" fill="currentColor" stroke="none"/></template>
      <template v-else-if="name === 'database'"><ellipse cx="12" cy="5.5" rx="7.5" ry="3"/><path d="M4.5 5.5v6c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3v-6M4.5 11.5v6c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3v-6"/></template>
      <template v-else-if="name === 'shield'"><path d="M12 3 5.5 5.5v5.7c0 4.3 2.7 7.8 6.5 9.8 3.8-2 6.5-5.5 6.5-9.8V5.5L12 3Z"/><path d="m9 12 2 2 4-4"/></template>
      <template v-else-if="name === 'clock'"><circle cx="12" cy="12" r="8.5"/><path d="M12 7v5l3.5 2"/></template>
      <template v-else-if="name === 'download'"><path d="M12 3v11m0 0 4-4m-4 4-4-4M4 18v2h16v-2"/></template>
      <template v-else-if="name === 'copy'"><rect x="8" y="8" width="11" height="11" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/></template>
      <template v-else-if="name === 'check'"><circle cx="12" cy="12" r="8.5"/><path d="m8.2 12 2.5 2.5 5.3-5.3"/></template>
      <template v-else-if="name === 'warning'"><path d="M12 3.5 21 20H3L12 3.5Z"/><path d="M12 9v5M12 17h.01"/></template>
      <template v-else-if="name === 'search'"><circle cx="10.5" cy="10.5" r="6.5"/><path d="m15.5 15.5 4.5 4.5"/></template>
      <template v-else-if="name === 'app'"><path d="M4 8.5 12 4l8 4.5v7L12 20l-8-4.5v-7Z"/><path d="m4 8.5 8 4.5 8-4.5M12 13v7"/></template>
      <template v-else-if="name === 'process'"><circle cx="8" cy="8" r="3"/><circle cx="16" cy="16" r="3"/><path d="M10.5 9.5 13.5 14.5M14.5 9.5 17 7M7 17l2.5-2.5"/></template>
      <template v-else-if="name === 'container'"><path d="m12 3 8 4.5v9L12 21l-8-4.5v-9L12 3Z"/><path d="m4 7.5 8 4.5 8-4.5M12 12v9"/></template>
      <template v-else-if="name === 'arrow-right'"><path d="M5 12h14m-5-5 5 5-5 5"/></template>
      <template v-else-if="name === 'arrow-left'"><path d="M19 12H5m5-5-5 5 5 5"/></template>
      <template v-else-if="name === 'restore'"><path d="M4 8v5h5"/><path d="M5 13a7.5 7.5 0 1 0 2-7"/></template>
      <template v-else-if="name === 'bell'"><path d="M6.5 16.5h11l-1.5-2V10a4 4 0 0 0-8 0v4.5l-1.5 2ZM10 19h4"/></template>
      <template v-else-if="name === 'file'"><path d="M7 3h7l4 4v14H7z"/><path d="M14 3v5h5M10 13h5M10 17h5"/></template>
      <template v-else-if="name === 'save'"><path d="M5 3h12l2 2v16H5z"/><path d="M8 3v6h8V3M8 21v-7h8v7"/></template>
      <template v-else-if="name === 'code'"><path d="m8 8-4 4 4 4M16 8l4 4-4 4M14 5l-4 14"/></template>
      <template v-else-if="name === 'chart'"><path d="M4 19V5M4 19h16"/><path d="m7 15 3-4 3 2 4-6"/></template>
      <template v-else-if="name === 'target'"><circle cx="12" cy="12" r="8.5"/><circle cx="12" cy="12" r="4.5"/><path d="m15 9 5-5M17 4h3v3"/></template>
      <template v-else><circle cx="12" cy="12" r="8.5"/></template>
    </g>
  </svg>
</template>
