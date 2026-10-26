<script setup lang="ts">
import { computed } from 'vue'
import type { StatisticsSnapshot } from '@/api/types'
import LineChart from '@/components/charts/LineChart.vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { statisticsColors } from './statisticsFormat'
const props = defineProps<{ remote: StatisticsSnapshot['remote'] }>()
const { t } = useI18n()
const config = computed(() => Object.fromEntries(props.remote.trafficSeries.map((series,index) => [`s${index}`, { label: series.name, color: statisticsColors[index % statisticsColors.length]! }])))
const data = computed(() => props.remote.trafficDates.map((date,index) => ({ x: index, label: date, ...Object.fromEntries(props.remote.trafficSeries.map((series,i) => [`s${i}`, Number(series.dailyBytes[index] ?? '0')])) })))
const hasTraffic = computed(() => props.remote.trafficSeries.some(series => series.dailyBytes.some(value => BigInt(value) > 0n)))
</script>
<template><section class="statistics-section"><div class="statistics-section__heading"><h2>{{ t('statistics.sevenDayTraffic') }}</h2></div><article v-if="hasTraffic" class="statistics-panel"><LineChart :data="data" :config="config" :label="t('statistics.trafficChartLabel')" :format-value="formatBytes" :format-x="(x: number) => Number.isInteger(x) ? data[x]?.label.slice(5) || '' : ''" bar stacked /></article><div v-else class="statistics-empty statistics-empty--panel">{{ t('statistics.noTraffic') }}</div></section></template>
