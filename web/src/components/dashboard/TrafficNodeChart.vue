<script setup lang="ts">
import { computed } from 'vue'
import LineChart from '@/components/charts/LineChart.vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
const props = defineProps<{ name: string; totalBytes: string; categories: readonly string[]; dailyBytes: readonly string[] }>()
const { t } = useI18n()
const data = computed(() => props.categories.map((date,index) => ({ x:index, label:date, bytes:Number(props.dailyBytes[index] ?? '0') })))
</script>
<template><LineChart :data="data" :config="{ bytes: { label: name, color: 'var(--accent)' } }" :label="t('home.trafficChartNode', { name, value: formatBytes(totalBytes) })" :format-value="formatBytes" :height="150" bar /></template>
