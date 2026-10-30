<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { XNodeMetrics, XNode } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import XNodeMetricRow from './XNodeMetricRow.vue'
import XNodeLogs from './XNodeLogs.vue'
import { useMetricSamples } from '@/composables/useMetricSamples'

const props = defineProps<{ metrics: XNodeMetrics | null; nodes?: XNode[] }>()
const { t } = useI18n()
type NodeMetric = XNodeMetrics['nodes'][number]
const liveNodes = computed(() => props.metrics?.nodes ?? [])
const names = computed(() => new Map(props.nodes?.map(node => [node.id, node.name]) ?? []))
const logging = shallowRef<{ id: string; name: string } | null>(null)
const logsOpen = computed({ get: () => logging.value !== null, set: open => { if (!open) logging.value = null } })
function viewLogs(node: NodeMetric) { logging.value = { id: node.nodeId, name: names.value.get(node.nodeId) ?? node.nodeId } }
const rate = (value?: number) => t('statistics.perSecond', { value: formatBytes(value ?? 0) })
const { series, append } = useMetricSamples()
watch(() => props.metrics, value => {
  if (!value) return
  const values: Record<string, number> = {}
  for (const node of value.nodes) {
    if (!node.available) continue
    const machine = node.machine as Record<string, unknown>
    const cpu = Array.isArray(machine.cpu_usage) ? machine.cpu_usage.filter((item): item is number => typeof item === 'number' && Number.isFinite(item)) : []
    const memory = machine.memory as { usedPercent?: number } | undefined
    if (cpu.length) values[`${node.nodeId}:cpu`] = cpu.reduce((sum, item) => sum + item, 0) / cpu.length
    if (typeof memory?.usedPercent === 'number') values[`${node.nodeId}:memory`] = memory.usedPercent
  }
  append(Date.parse(value.fetchedAt), values)
}, { immediate: true })
</script>

<template>
  <div class="grid gap-5">
    <div v-if="metrics" class="metrics-summary"><div><small>{{ t('xnode.onlineNodes') }}</small><strong>{{ metrics.onlineNodes }} / {{ metrics.totalNodes }}</strong></div><div><small>{{ t('xnode.connections') }}</small><strong>{{ metrics.totalConnections ?? t('common.notAvailable') }}</strong></div><div><small>{{ t('xnode.upload') }}</small><strong>{{ rate(metrics.uploadBytesPerSecond) }}</strong></div><div><small>{{ t('xnode.download') }}</small><strong>{{ rate(metrics.downloadBytesPerSecond) }}</strong></div></div>
    <p v-if="!liveNodes.length" class="text-muted">{{ t('xnode.noMetrics') }}</p>
    <XNodeMetricRow v-for="node in liveNodes" :key="node.nodeId" :node="node" :name="names.get(node.nodeId) ?? node.nodeId" :cpu="series[`${node.nodeId}:cpu`] ?? []" :memory="series[`${node.nodeId}:memory`] ?? []" @logs="viewLogs(node)" />
    <XNodeLogs v-if="logging" v-model:open="logsOpen" :node-id="logging.id" :name="logging.name" />
  </div>
</template>

<style scoped>
.metrics-summary { display: grid; grid-template-columns: repeat(4,minmax(0,1fr)); gap: 16px; }
.metrics-summary div { display: grid; gap: 4px; }
small, .metrics-meta { color: var(--text-muted); font-size: 12px; }
strong { font-size: 20px; }
@media(max-width:600px) { .metrics-summary { grid-template-columns: minmax(0,1fr); } }
</style>
