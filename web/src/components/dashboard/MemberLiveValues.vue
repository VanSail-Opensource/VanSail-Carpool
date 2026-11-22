<script setup lang="ts">
import { computed } from 'vue'
import type { DeepReadonly } from '@/api/types'
import type { XNodeUserMetrics } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import MemberMetricValue from './MemberMetricValue.vue'

const props = defineProps<{ metrics: DeepReadonly<XNodeUserMetrics> | null }>()
const { t } = useI18n()
const rates = computed(() => [
  { key: 'download', icon: 'i-ph-arrow-down', bytes: props.metrics?.downloadBytesPerSecond },
  { key: 'upload', icon: 'i-ph-arrow-up', bytes: props.metrics?.uploadBytesPerSecond },
].map(rate => {
  const [value, unit] = rate.bytes != null && Number.isFinite(rate.bytes) && rate.bytes >= 0 ? formatBytes(rate.bytes).split(' ') : []
  return { ...rate, value: value ?? null, unit: unit ? t('statistics.perSecond', { value: unit }) : undefined }
}))
const connections = computed(() => [
  { key: 'activeConnections', value: props.metrics ? String(props.metrics.connections) : null },
  { key: 'openConnections', value: props.metrics?.totalConnections ?? null },
])
</script>

<template>
  <div class="member-live-values">
    <dl class="member-live-values__rates">
      <div v-for="rate in rates" :key="rate.key" class="member-live-values__rate" :data-metric="rate.key">
        <dt><UIcon :name="rate.icon" aria-hidden="true" />{{ t(`xnode.${rate.key}`) }}</dt>
        <dd><MemberMetricValue :value="rate.value" :unit="rate.unit" /></dd>
      </div>
    </dl>
    <dl class="member-live-values__connections">
      <div v-for="count in connections" :key="count.key" :data-metric="count.key">
        <dt>{{ t(`memberNodes.${count.key}`) }}</dt>
        <dd><MemberMetricValue :value="count.value" /></dd>
      </div>
    </dl>
    <p class="member-live-values__hint">{{ t('memberNodes.activeConnectionsHint') }}</p>
  </div>
</template>

<style scoped>
.member-live-values { container: live-values / inline-size; }
.member-live-values__rates, .member-live-values__connections { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .8rem 1.25rem; margin: 0; }
.member-live-values__rate, .member-live-values__connections > div { min-width: 0; }
.member-live-values dt { display: flex; align-items: center; gap: .35rem; color: var(--text-muted); font-size: .75rem; overflow-wrap: anywhere; }
.member-live-values dd { margin: .35rem 0 0; color: var(--text); font-size: 1.65rem; font-weight: 600; line-height: 1.25; }
.member-live-values__rates dt > :first-child { flex: 0 0 auto; color: var(--accent); }
.member-live-values__connections { margin-top: 1rem; padding-top: .85rem; border-top: 1px solid var(--line); }
.member-live-values__connections dd { font-size: 1.05rem; }
.member-live-values__hint { margin: .75rem 0 0; color: var(--text-faint); font-size: .7rem; line-height: 1.5; }
@container live-values (min-width: 28rem) {
  .member-live-values__connections > div { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: .25rem .65rem; }
  .member-live-values__connections dd { margin: 0; }
}
@container live-values (max-width: 15rem) {
  .member-live-values__connections { grid-template-columns: minmax(0, 1fr); }
}
</style>
