<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { XNodeMetrics } from '@/api/xnode'
import type { MetricSample } from '@/composables/useMetricSamples'
import { useI18n } from '@/i18n'
import { formatDate, formatBytes } from '@/utils/format'
import MetricLineChart from '@/components/common/MetricLineChart.vue'
import XNodeMetricSection from './XNodeMetricSection.vue'
import { metricLabel } from './metricRows'
const props = defineProps<{ node: XNodeMetrics['nodes'][number]; name: string; cpu: MetricSample[]; memory: MetricSample[] }>()
const emit = defineEmits<{ logs: [] }>()
const { t } = useI18n()
const expanded = shallowRef(false)
const status = computed(() => {
  const node = props.node
  if (node.stale) return { label: 'xnode.stale', color: 'neutral' as const }
  if (!node.connected || !node.ready) return { label: 'xnode.offline', color: 'error' as const }
  if (!node.synchronized) return { label: 'xnode.degraded', color: 'warning' as const }
  if (!node.available) return { label: 'xnode.metricsUnavailable', color: 'neutral' as const }
  return { label: 'xnode.online', color: 'success' as const }
})
const fields = computed(() => expanded.value ? Object.entries(props.node.machine ?? {}) : [])
const rate = (value?: number) => t('statistics.perSecond', { value: formatBytes(value ?? 0) })
function toggle(event: globalThis.Event) { expanded.value = (event.currentTarget as InstanceType<typeof globalThis.HTMLDetailsElement>).open }
</script>

<template>
  <details class="node-metrics" @toggle="toggle">
    <summary><span class="min-w-0 flex-1 break-words">{{ name }}</span><UBadge :color="status.color" variant="subtle">{{ t(status.label) }}</UBadge><UButton type="button" variant="ghost" color="neutral" icon="i-ph-terminal-window" :aria-label="t('xnode.viewLogs', { name })" :title="t('xnode.viewLogs', { name })" @click.stop.prevent="emit('logs')" /></summary>
    <template v-if="expanded">
      <div class="metrics-meta"><span>{{ node.version }} · Xray {{ node.xrayVersion }}</span><span>{{ formatDate(node.reportedAt ?? undefined) }}</span><span v-if="node.available">{{ rate(node.uploadBytesPerSecond) }} ↑ · {{ rate(node.downloadBytesPerSecond) }} ↓</span></div>
      <div v-if="node.available" class="metrics-sections"><MetricLineChart :samples="cpu" :label="t('adminNative.cpu')" unit="percent" /><MetricLineChart :samples="memory" :label="t('adminNative.memory')" unit="percent" /></div>
      <div v-if="node.available" class="metrics-sections"><XNodeMetricSection v-for="[key, value] in fields" :key="key" :value="value" :label="metricLabel(key)" :prefix="key" /><XNodeMetricSection :value="node.certificates" :label="t('xnode.certificates')" /></div>
    </template>
  </details>
</template>

<style scoped>
.node-metrics { border-bottom: 1px solid var(--line); padding-bottom: 12px; min-width: 0; }
summary { display: flex; align-items: center; justify-content: space-between; gap: 12px; cursor: pointer; padding-block: 8px; }
.metrics-meta { color: var(--text-muted); font-size: 12px; display: flex; gap: 12px; flex-wrap: wrap; }
.metrics-sections { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 8px 24px; padding-top: 12px; }
@media(max-width:600px) { .metrics-sections { grid-template-columns: minmax(0,1fr); } summary { gap: 8px; } }
</style>
