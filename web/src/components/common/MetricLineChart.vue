<script setup lang="ts">
import { computed } from 'vue'
import type { MetricSample } from '@/composables/useMetricSamples'
import LineChart from '@/components/charts/LineChart.vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
const props = defineProps<{ samples: readonly MetricSample[]; label: string; unit?: 'bytes' | 'percent' | 'count' | 'money'; forecasts?: readonly { at: number; value: number; low: number; high: number }[]; events?: readonly { at: number; label: string }[]; showBand?: boolean; connectMissing?: boolean }>()
const { t, locale } = useI18n()
const data = computed(() => {
 const rows = new Map<number, { x: number; label: string; actual?: number; predicted?: number; low?: number; high?: number }>()
 for (const p of props.samples.slice(-300)) rows.set(p.at, { x: p.at, label: new Date(p.at).toLocaleString(locale.value), actual: Number.isFinite(p.value) ? p.value : undefined })
 for (const p of props.forecasts || []) rows.set(p.at, { ...rows.get(p.at), x: p.at, label: new Date(p.at).toLocaleString(locale.value), predicted: p.value, low: p.low, high: p.high })
 return [...rows.values()].sort((a,b) => a.x-b.x)
})
const config = computed(() => ({ actual: { label: props.label, color: 'var(--accent)' }, ...(props.forecasts?.length ? { predicted: { label: t('predictions.predicted'), color: 'var(--text-muted)', dashed: true } } : {}) }))
function formatTime(at: number): string {
 const span = (data.value.at(-1)?.x ?? 0) - (data.value[0]?.x ?? 0)
 return new Intl.DateTimeFormat(locale.value, span >= 86400000 ? { month: 'short', day: 'numeric', timeZone: 'UTC' } : { hour: '2-digit', minute: '2-digit' }).format(at)
}
function format(value: number): string { return props.unit === 'percent' ? `${value.toFixed(1)}%` : props.unit === 'money' ? `${(value / 100).toFixed(2)} TXB` : props.unit === 'count' ? String(Math.round(value)) : props.unit === 'bytes' ? formatBytes(value) : t('statistics.perSecond', { value: formatBytes(value) }) }
</script>
<template>
  <figure class="metric-chart"><figcaption>{{ label }}</figcaption><LineChart v-if="data.length" :data="data" :config="config" :label="label" :height="160" :format-value="format" :format-x="formatTime" :band="showBand ? { low: 'low', high: 'high', color: 'var(--accent)' } : undefined" :connect-missing="connectMissing" :bridge="connectMissing && samples.length && forecasts?.length ? { from: 'actual', to: 'predicted', color: 'var(--text-muted)' } : undefined" /><p v-else>{{ t('xnode.noMetrics') }}</p><ul v-if="events?.length" class="metric-chart__events"><li v-for="(event,index) in events" :key="index">{{ new Date(event.at).toLocaleString(locale) }} · {{ event.label }}</li></ul></figure>
</template>
<style scoped>
.metric-chart { min-width: 0; margin: 0; display: grid; gap: .5rem; }.metric-chart figcaption { font-size: .8rem; font-weight: 600; }.metric-chart__events { padding-inline-start: 1rem; font-size: .7rem; color: var(--text-muted); }
</style>
