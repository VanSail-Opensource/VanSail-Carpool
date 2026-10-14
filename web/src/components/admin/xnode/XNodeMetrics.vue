<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeMetrics } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { formatDate } from '@/utils/format'

const props = defineProps<{ metrics: XNodeMetrics | null }>()
const { t } = useI18n()
const nodes = computed(() => props.metrics?.nodes ?? [])
const rate = (value?: number) => `${((value ?? 0) * 8 / 1_000_000).toFixed(2)} ${t('xnode.mbps')}`
const fields = (value: unknown): [string, unknown][] => value && typeof value === 'object' ? Object.entries(value) : []
</script>

<template>
  <div class="grid gap-5">
    <div v-if="metrics" class="metrics-summary"><div><small>{{ t('xnode.onlineNodes') }}</small><strong>{{ metrics.onlineNodes }} / {{ metrics.totalNodes }}</strong></div><div><small>{{ t('xnode.connections') }}</small><strong>{{ metrics.activeConnections }}</strong></div><div><small>{{ t('xnode.upload') }}</small><strong>{{ rate(metrics.uploadBytesPerSecond) }}</strong></div><div><small>{{ t('xnode.download') }}</small><strong>{{ rate(metrics.downloadBytesPerSecond) }}</strong></div></div>
    <p v-if="!nodes.length" class="text-muted">{{ t('xnode.noMetrics') }}</p>
    <details v-for="node in nodes" :key="node.nodeId" class="node-metrics">
      <summary><span>{{ node.nodeId }}</span><UBadge :color="node.stale ? 'neutral' : 'success'" variant="subtle">{{ t(node.stale ? 'xnode.stale' : 'xnode.online') }}</UBadge></summary>
      <div class="metrics-meta"><span>{{ node.version }} · Xray {{ node.xrayVersion }}</span><span>{{ formatDate(node.reportedAt ?? undefined) }}</span><span>{{ rate(node.uploadBytesPerSecond) }} ↑ · {{ rate(node.downloadBytesPerSecond) }} ↓</span></div>
      <div class="metrics-sections"><details v-for="[key, value] in fields(node.machine)" :key="key"><summary>{{ t(`xnode.collectors.${key}`) }}</summary><pre>{{ JSON.stringify(value, null, 2) }}</pre></details></div>
    </details>
  </div>
</template>

<style scoped>
.metrics-summary { display: grid; grid-template-columns: repeat(4,minmax(0,1fr)); gap: 16px; }
.metrics-summary div { display: grid; gap: 4px; }
small, .metrics-meta { color: var(--text-muted); font-size: 12px; }
strong { font-size: 20px; }
.node-metrics { border-bottom: 1px solid var(--line); padding-bottom: 12px; }
summary { display: flex; align-items: center; justify-content: space-between; gap: 12px; cursor: pointer; padding-block: 8px; }
.metrics-meta { display: flex; gap: 12px; flex-wrap: wrap; }
pre { font-size: 12px; overflow: auto; max-height: 340px; }
.metrics-sections { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 8px 24px; padding-top: 12px; }
@media(max-width:600px) { .metrics-summary,.metrics-sections { grid-template-columns: repeat(2,minmax(0,1fr)); } }
</style>
