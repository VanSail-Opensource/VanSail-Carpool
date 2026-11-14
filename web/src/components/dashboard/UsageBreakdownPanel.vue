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
  <section class="section-block usage-breakdown-panel grid gap-3 min-w-0" :aria-busy="loading || refreshing" data-usage-panel>
    <header class="usage-breakdown-heading"><h2>{{ t('usageBreakdown.title') }}</h2><UsageBreakdownMeasure v-model="metric" class="usage-breakdown-heading__measure" /><UButton class="usage-breakdown-heading__refresh" variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="refreshing" data-haptic="refresh" @click="refresh()" /></header>
    <div v-if="loading" role="status" aria-live="polite"><span class="sr-only">{{ t('common.loading') }}</span><UProgress aria-hidden="true" /></div>
    <UAlert v-if="error" color="warning" :title="value ? t('usageBreakdown.stale') : error" :description="value ? error : undefined" />
    <UAlert v-else-if="stale" color="warning" :title="t('usageBreakdown.stale')" />
    <UsageBreakdownPie v-if="value" v-model:metric="metric" :history="value" hide-measure />
  </section>
</template>

<style scoped>
.usage-breakdown-panel { container: usage-breakdown / inline-size; }
.usage-breakdown-heading { display: grid; grid-template-columns: minmax(0, 1fr) auto; grid-template-areas: 'title title' 'measure refresh'; align-items: center; gap: .5rem; min-width: 0; }
.usage-breakdown-heading h2 { grid-area: title; margin: 0; min-width: 0; font-size: .95rem; line-height: 1.25; overflow-wrap: normal; }
.usage-breakdown-heading__measure { grid-area: measure; justify-self: start; }
.usage-breakdown-heading__refresh { grid-area: refresh; }
@container usage-breakdown (min-width: 24rem) {
  .usage-breakdown-heading { grid-template-columns: minmax(0, 1fr) auto auto; grid-template-areas: 'title measure refresh'; }
}
</style>
