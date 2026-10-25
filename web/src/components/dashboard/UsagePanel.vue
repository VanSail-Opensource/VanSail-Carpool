<script setup lang="ts">
import { computed } from 'vue'

import type { CatalogNode, RFC3339, UsageStatistics } from '@/api/types'
import InlineNotice from '@/components/common/InlineNotice.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { formatBytes, formatDateTime } from '@/utils/format'
import TrafficUsageBar from './TrafficUsageBar.vue'
import type { ActivityStatus } from './memberActivity'

const props = withDefaults(defineProps<{
  activity?: ActivityStatus
  statistics: UsageStatistics
  ratio: number
  stale: boolean
  fetchedAt: RFC3339
  catalogNodes?: readonly CatalogNode[]
}>(), { activity: 'unavailable', catalogNodes: () => [] })

const percentage = computed(() => Math.min(100, Math.max(0, Math.round(props.ratio * 100))))
const tone = computed(() => percentage.value >= 90 ? 'danger' : percentage.value >= 75 ? 'warning' : 'safe')
const progressColor = computed(() => tone.value === 'danger' ? 'error' : tone.value === 'warning' ? 'warning' : 'success')
const remainingTraffic = computed(() => {
  try {
    const remaining = BigInt(props.statistics.trafficLimitBytes) - BigInt(props.statistics.usedTrafficBytes)
    return formatBytes((remaining > 0n ? remaining : 0n).toString())
  } catch {
    return formatBytes('0')
  }
})
</script>

<template>
  <section class="section-block home-usage" :class="`home-usage--${tone}`">
    <div class="section-heading">
      <h2>{{ $t('dashboard.usage') }}</h2>
      <StatusBadge :tone="activity === 'online' ? 'success' : activity === 'offline' ? 'danger' : 'neutral'" :label="$t(activity === 'online' ? 'dashboard.online' : activity === 'offline' ? 'dashboard.offline' : 'dashboard.activityUnavailable')" />
    </div>

    <InlineNotice v-if="stale" tone="warning">
      {{ $t('dashboard.lastKnownData') }} · {{ $t('dashboard.usageUnavailable') }} {{ $t('dashboard.showingDataFrom', { date: formatDateTime(fetchedAt) }) }}
    </InlineNotice>

    <div class="home-usage__summary">
      <strong class="home-usage__percentage">{{ percentage }}%</strong>
    </div>
    <div class="home-usage__progress-group" role="group" :aria-label="$t('dashboard.usage')">
      <UProgress class="home-usage__meter" :model-value="percentage" :max="100" :color="progressColor" />
      <dl class="home-usage__progress-details">
        <div><dt>{{ $t('dashboard.usedTraffic') }}</dt><dd>{{ formatBytes(statistics.usedTrafficBytes) }}</dd></div>
        <div><dt>{{ $t('dashboard.remainingTraffic') }}</dt><dd>{{ remainingTraffic }}</dd></div>
      </dl>
    </div>
    <TrafficUsageBar
      :nodes="statistics.topNodes"
      :total-bytes="statistics.usedTrafficBytes"
      :catalog-nodes="catalogNodes"
      :use-multiplier="false"
    />
    <dl class="home-usage__last-reset">
      <dt>{{ $t('dashboard.lastTrafficReset') }}</dt>
      <dd>{{ statistics.lastTrafficResetAt ? formatDateTime(statistics.lastTrafficResetAt) : $t('dashboard.noTrafficReset') }}</dd>
    </dl>
  </section>
</template>

<style scoped>
.home-usage__last-reset { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 0.35rem 0.75rem; margin: 0.8rem 0 0; padding-top: 0.7rem; border-top: 1px solid var(--line); color: var(--text-muted); font-size: 0.75rem; }
.home-usage__last-reset dd { margin: 0; }
</style>
