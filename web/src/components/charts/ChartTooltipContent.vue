<script setup lang="ts">
import { computed } from 'vue'
import type { ChartConfig } from './index'
const props = defineProps<{ config: ChartConfig; payload?: Record<string, unknown>; x?: number | Date; labelFormatter?: (x: number | Date) => string; valueFormatter?: (value: number) => string; missingLabel?: string }>()
const entries = computed(() => Object.keys(props.config).map(key => [key, props.payload?.[key]] as const).filter(([, value]) => props.missingLabel !== undefined || typeof value === 'number' && Number.isFinite(value)))
</script>
<template>
  <div class="chart-tooltip">
    <strong>{{ labelFormatter && x !== undefined ? labelFormatter(x) : payload?.label }}</strong>
    <div v-for="[key, value] in entries" :key="key"><span :style="{ color: config[key]?.color }">{{ config[key]?.label }}</span><b>{{ typeof value === 'number' && Number.isFinite(value) ? (valueFormatter ? valueFormatter(value) : value) : missingLabel }}</b></div>
  </div>
</template>
<style scoped>
.chart-tooltip { display: grid; gap: .4rem; min-width: 9rem; padding: .65rem; color: var(--text); background: var(--surface); border: 1px solid var(--line); border-radius: .5rem; box-shadow: 0 4px 16px #0002; }
.chart-tooltip div { display: flex; justify-content: space-between; gap: 1rem; }
</style>
