<script setup lang="ts">
import { useChart } from './index'
const config = useChart()
</script>
<template><ul class="chart-legend"><li v-for="(item, key) in config" :key="key"><span :style="{ borderColor: item.color, borderTopStyle: item.dashed ? 'dashed' : 'solid' }" />{{ item.label }}</li></ul></template>
<style scoped>
.chart-legend { display: flex; flex-wrap: wrap; gap: .5rem 1rem; list-style: none; margin: .5rem 0; padding: 0; color: var(--text-muted); }
.chart-legend li { display: flex; align-items: center; gap: .35rem; }.chart-legend span { width: 1rem; border-top-width: 2px; }
</style>
