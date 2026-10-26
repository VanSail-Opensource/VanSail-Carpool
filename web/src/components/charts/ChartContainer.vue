<script setup lang="ts">
import { computed, toRef, useId } from 'vue'
import type { ChartConfig } from './index'
import { provideChartContext } from './index'
const props = defineProps<{ config: ChartConfig }>()
const id = useId()
provideChartContext(toRef(props, 'config'))
const colors = computed(() => Object.fromEntries(Object.entries(props.config).map(([key, item]) => [`--color-${key}`, item.color])))
</script>
<template>
  <div data-slot="chart" :data-chart="id" class="chart-container" :style="colors"><slot /></div>
</template>
<style scoped>
.chart-container { width: 100%; min-width: 0; font-size: .75rem; --vis-font-family: inherit; --vis-axis-tick-label-color: var(--text-muted); --vis-axis-grid-color: var(--line); --vis-axis-domain-line-color: var(--line); --vis-donut-background-color: var(--surface); --vis-donut-central-label-text-color: var(--text); --vis-donut-central-sub-label-text-color: var(--text-muted); --vis-tooltip-padding: 0; --vis-tooltip-background-color: transparent; --vis-tooltip-border-color: transparent; --vis-tooltip-shadow-color: transparent; --vis-crosshair-line-stroke-color: var(--text-muted); }
.chart-container :deep([data-vis-xy-container]), .chart-container :deep([data-vis-single-container]) { width: 100%; }
.chart-container :deep(.tick text) { fill: var(--text-muted); }
</style>
