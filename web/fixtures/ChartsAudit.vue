<script setup lang="ts">
import StatisticsDonut from '@/components/statistics/StatisticsDonut.vue'
import StatisticsPie from '@/components/statistics/StatisticsPie.vue'
import StatisticsConcentricDonut from '@/components/statistics/StatisticsConcentricDonut.vue'
import StatisticsTrafficChart from '@/components/statistics/StatisticsTrafficChart.vue'
import MetricLineChart from '@/components/common/MetricLineChart.vue'
import LineChart from '@/components/charts/LineChart.vue'
import type { StatisticsSnapshot } from '@/api/types'
import { useI18n } from '@/i18n'
const { t } = useI18n()
const params = new URLSearchParams(location.search)
const exact = params.has('exact')
const items = [{ id: 'active', label: 'Active', value: 3 }, { id: 'queued', label: 'Queued', value: 1 }]
const remote: StatisticsSnapshot['remote'] = { weeklyUserIncrease: 0, monthlyAverageUsagePercent: 0,
  trafficDates: exact ? ['2026-10-24', '2026-10-25', '2026-10-26'] : ['2026-10-24', '2026-10-25'],
  trafficSeries: exact ? [{ uuid: 'accounted', name: t('adminNative.weighted'), dailyBytes: params.has('invalid') ? ['invalid', '1'] : ['9007199254740993', '10000', '0'] }]
    : [{ uuid: 'Tokyo', name: 'Tokyo', dailyBytes: ['1073741824', '2147483648'] }, { uuid: 'Frankfurt', name: 'Frankfurt', dailyBytes: ['2147483648', '1073741824'] }],
}
const samples = [{ at: Date.UTC(2026, 9, 24), value: 2 ** 53 }, { at: Date.UTC(2026, 9, 25), value: 10000 }]
</script>
<template>
  <section class="charts-audit">
    <StatisticsDonut :items="items" :center-label="t('adminUserProfile.entitlements')" center-value="4" :chart-label="t('statistics.subscriptionStates')" />
    <StatisticsPie :items="items" :chart-label="t('statistics.subscriptionStates')" />
    <StatisticsConcentricDonut :rings="[{ id: 'first', label: 'First', items }, { id: 'second', label: 'Second', items }]" :center-label="t('adminUserProfile.entitlements')" center-value="4" :chart-label="t('statistics.subscriptionStates')" />
    <StatisticsTrafficChart data-test="traffic-precision" :remote="remote" />
    <MetricLineChart v-if="exact" data-test="metric-precision" :samples="samples" :label="t('adminNative.lifetime')" unit="bytes" />
    <LineChart :data="[{ x: 0, label: 'First', a: 2, b: 1 }, { x: 1, label: 'Second', a: 1, b: 3 }]" :config="{ a: { label: 'Tokyo', color: 'var(--accent)' }, b: { label: 'Frankfurt', color: 'var(--text-muted)' } }" :label="t('statistics.squadComposition')" bar stacked />
  </section>
</template>
<style scoped>.charts-audit { display: grid; gap: 2rem; }</style>
