<script setup lang="ts">
import type { VersionInfo } from '../api'
import GlassPanel from '../components/GlassPanel.vue'
import LocalIcon from '../components/LocalIcon.vue'

defineProps<{ version: VersionInfo; iconUrl: string }>()

const capabilities = ['识别物理磁盘', '记录状态变化和唤醒事件', '提供带证据的疑似来源分析', '导出事件和脱敏诊断报告']
const limitations = ['机械盘状态探测可能显示未知', '疑似来源仅表示时间相关性', 'SSD/NVMe 不适用机械盘待机状态', '部分硬件能力需要 fnOS 真机验证']
</script>

<template>
  <GlassPanel class="about-hero">
    <img :src="iconUrl" alt="硬盘唤醒追踪器图标" />
    <h2>硬盘唤醒追踪器</h2>
    <p class="english-title">DISK OBSERVABILITY</p>
    <span class="version-pill">v{{ version.version }}</span>
    <p>一款 fnOS 原生系统工具，以低干扰方式观察机械硬盘状态变化，帮助了解硬盘何时被唤醒，以及当时出现的疑似 I/O 来源。</p>
    <dl class="build-meta" v-if="version.commit || version.buildTime || version.platform">
      <div v-if="version.commit"><dt>提交</dt><dd>{{ version.commit }}</dd></div>
      <div v-if="version.buildTime"><dt>构建时间</dt><dd>{{ version.buildTime }}</dd></div>
      <div v-if="version.platform"><dt>平台</dt><dd>{{ version.platform }}</dd></div>
    </dl>
  </GlassPanel>
  <section class="about-grid">
    <GlassPanel><div class="panel-heading"><div><small>真实功能范围</small><h2>核心能力</h2></div><LocalIcon name="target" /></div><ul class="feature-list"><li v-for="item in capabilities" :key="item"><LocalIcon name="check" />{{ item }}</li></ul></GlassPanel>
    <GlassPanel><div class="panel-heading"><div><small>谨慎解释结果</small><h2>当前限制</h2></div><LocalIcon name="warning" /></div><ul class="feature-list limitations"><li v-for="item in limitations" :key="item"><LocalIcon name="info" />{{ item }}</li></ul></GlassPanel>
  </section>
</template>
