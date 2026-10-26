<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { StatisticsSnapshot } from '@/api/types'
import LineChart from '@/components/charts/LineChart.vue'
import { useI18n } from '@/i18n'
import { statisticsColors } from './statisticsFormat'
const props = defineProps<{ database: StatisticsSnapshot['database'] }>()
const { t } = useI18n()
const mode = shallowRef('squad')
const tabs = computed(() => [{ label:t('statistics.bySquad'),value:'squad' },{ label:t('statistics.byCombo'),value:'combo' }])
const rows = computed(() => mode.value === 'squad' ? props.database.comboBySquad : props.database.squadByCombo)
const legend = computed(() => [...new Map(rows.value.flatMap(row => row.segments.map(segment => [segment.id,segment.label] as const))).entries()])
const config = computed(() => Object.fromEntries(legend.value.map(([id,label],index) => [id,{ label,color:statisticsColors[index % statisticsColors.length]! }])))
const data = computed(() => rows.value.map((row,index) => {
 const total = row.segments.reduce((sum,s) => sum + s.value,0)
 return { x:index,label:row.label,...Object.fromEntries(row.segments.map(s => [s.id,total ? s.value / total * 100 : 0])) }
}))
</script>
<template><section class="statistics-section"><div class="statistics-distribution-heading"><h2>{{ t('statistics.squadComposition') }}</h2><UTabs v-model="mode" :items="tabs" :content="false" size="sm" /></div><article v-if="rows.length" class="statistics-panel"><LineChart :data="data" :config="config" :label="t('statistics.squadComposition')" :format-value="(v: number) => `${v.toFixed(1)}%`" bar stacked /></article><p v-else class="statistics-empty">{{ t('statistics.noData') }}</p></section></template>
