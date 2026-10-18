<script setup lang="ts">
import { computed } from 'vue'
import type { MetricSample } from '@/composables/useMetricSamples'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
const props = defineProps<{ samples: readonly MetricSample[]; label: string; unit?: 'bytes' | 'percent' | 'count' }>()
const { t } = useI18n()
const visible = computed(() => props.samples.slice(-300).filter(item => Number.isFinite(item.value)))
const maximum = computed(() => props.unit === 'percent' ? 100 : Math.max(1, ...visible.value.map(item => item.value)))
const points = computed(() => visible.value.map((item, index) => `${8 + index / Math.max(1, visible.value.length - 1) * 624},${132 - item.value / maximum.value * 120}`).join(' '))
const last = computed(() => visible.value.at(-1)?.value ?? 0)
const formatted = computed(() => !visible.value.length ? t('common.notAvailable') : props.unit === 'percent' ? `${last.value.toFixed(1)}%` : props.unit === 'count' ? String(Math.round(last.value)) : props.unit === 'bytes' ? formatBytes(last.value) : t('statistics.perSecond', { value: formatBytes(last.value) }))
</script>
<template>
  <figure class="metric-chart">
    <figcaption><span>{{ label }}</span><strong>{{ formatted }}</strong></figcaption>
    <svg v-if="visible.length" viewBox="0 0 640 144" role="img" :aria-label="label" preserveAspectRatio="none"><path d="M8 132 H632" class="metric-chart__axis" /><polyline :points="points" fill="none" stroke="currentColor" stroke-width="2" vector-effect="non-scaling-stroke" /></svg>
    <p v-else class="text-muted">{{ t('xnode.noMetrics') }}</p>
  </figure>
</template>
<style scoped>
.metric-chart { min-width: 0; margin: 0; display: grid; gap: .5rem; }
figcaption { display: flex; justify-content: space-between; gap: .5rem; font-size: .75rem; color: var(--text-muted); }
svg { width: 100%; height: 112px; color: var(--accent); }
.metric-chart__axis { stroke: var(--line); fill: none; }
</style>
