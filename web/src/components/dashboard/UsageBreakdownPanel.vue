<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { usageBreakdownApi } from '@/api/usageBreakdown'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { t } from '@/i18n'
import UsageBreakdownPie from './UsageBreakdownPie.vue'
import UsageBreakdownMeasure from './UsageBreakdownMeasure.vue'
import type { UsageBreakdownMetric } from './usageBreakdownSegments'

const { value, loading, refreshing, error, refresh } = useSubscriptionData(usageBreakdownApi.get, 60_000, 'usageBreakdown.loadFailed')
const metric = shallowRef<UsageBreakdownMetric>('traffic')
const stale = computed(() => !!value.value && Date.now() - new Date(value.value.to).getTime() > 120_000)
</script>

<template>
  <section class="section-block grid gap-3 min-w-0" :aria-busy="loading || refreshing" data-usage-panel>
    <header class="usage-breakdown-heading"><h2>{{ t('usageBreakdown.title') }}</h2><UsageBreakdownMeasure v-model="metric" /><UButton class="usage-breakdown-heading__refresh" variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="refreshing" data-haptic="refresh" @click="refresh()" /></header>
    <div v-if="loading" role="status" aria-live="polite"><span class="sr-only">{{ t('common.loading') }}</span><UProgress aria-hidden="true" /></div>
    <UAlert v-if="error" color="warning" :title="value ? t('usageBreakdown.stale') : error" :description="value ? error : undefined" />
    <UAlert v-else-if="stale" color="warning" :title="t('usageBreakdown.stale')" />
    <UsageBreakdownPie v-if="value" v-model:metric="metric" :history="value" hide-measure />
  </section>
</template>

<style scoped>
.usage-breakdown-heading { display: flex; align-items: center; gap: .5rem; min-width: 0; }
.usage-breakdown-heading h2 { margin: 0; margin-right: auto; min-width: 0; font-size: .95rem; line-height: 1.25; }
.usage-breakdown-heading__refresh { flex: 0 0 auto; }
@media (max-width: 399px) {
  .usage-breakdown-heading { flex-wrap: wrap; gap: .25rem; }
  .usage-breakdown-heading h2 { flex: 1 1 min-content; font-size: .85rem; }
  .usage-breakdown-heading__refresh { margin-left: auto; }
}
</style>
