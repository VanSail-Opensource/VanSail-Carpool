<script setup lang="ts">
import { computed } from 'vue'
import type { StatisticsSnapshot } from '@/api/types'
import type { ChartDatum } from '@/components/charts'
import LineChart from '@/components/charts/LineChart.vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { statisticsColors } from './statisticsFormat'
const props = defineProps<{ remote: StatisticsSnapshot['remote'] }>()
const { t } = useI18n()
const config = computed(() => Object.fromEntries(props.remote.trafficSeries.map((series,index) => [`s${index}`, { label: series.name, color: statisticsColors[index % statisticsColors.length]! }])))
const data = computed<ChartDatum[]>(() => props.remote.trafficDates.map((date, index) => ({
  x: index, label: date, ...Object.fromEntries(props.remote.trafficSeries.flatMap((series, i) => {
    const raw = series.dailyBytes[index]
    const value = typeof raw === 'string' && /^\d+$/.test(raw) ? Number(raw) : NaN
    return [[`s${i}`, Number.isFinite(value) ? value : undefined], [`s${i}Bytes`, Number.isFinite(value) ? raw : undefined]]
  })),
})))
const hasTraffic = computed(() => data.value.some(row => Object.keys(config.value).some(key => typeof row[key] === 'number' && row[key] > 0)))
const incomplete = computed(() => data.value.some(row => Object.keys(config.value).some(key => row[key] === undefined)))
function detail(row: ChartDatum, key: string): string {
  const raw = row[`${key}Bytes`]
  return typeof raw === 'string' ? `${formatBytes(raw)} · ${t('adminNative.exactBytes', { bytes: raw })}` : t('common.notAvailable')
}
</script>
<template>
  <section class="statistics-section">
    <div class="statistics-section__heading"><h2>{{ t('statistics.sevenDayTraffic') }}</h2></div>
    <article v-if="hasTraffic" class="statistics-panel"><LineChart :data="data" :config="config" :label="t('statistics.trafficChartLabel')" :format-value="formatBytes" :format-datum-value="detail" :format-x="(x: number) => Number.isInteger(x) ? data[x]?.label.slice(5) || '' : ''" bar stacked /></article>
    <div v-else class="statistics-empty statistics-empty--panel">{{ t(incomplete ? 'common.notAvailable' : 'statistics.noTraffic') }}</div>
  </section>
</template>
