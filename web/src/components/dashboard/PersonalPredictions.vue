<script setup lang="ts">
import { computed } from 'vue'
import { forecastApi } from '@/api/forecasts'
import LineChart from '@/components/charts/LineChart.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { useI18n } from '@/i18n'
import { formatBytes, formatAccountingDay } from '@/utils/format'
const props = defineProps<{ userId?: string; back?: boolean }>()
const emit = defineEmits<{ back: [] }>()
const { t } = useI18n()
const { value: forecast, loading, error, refresh } = useSubscriptionData(signal => props.userId ? forecastApi.user(props.userId, signal) : forecastApi.personal(signal), 15_000, 'predictions.loadFailed', value => value?.refreshPending || value?.renewalDecision?.status === 'PENDING' ? 15_000 : (Math.floor(Date.now() / 86400000) + 1) * 86400000 - Date.now())
const data = computed(() => {
  const days = (forecast.value?.days || []).map(day => ({ x: Date.parse(day.date), label: day.date, actual: day.actualBytes === null ? undefined : Number(day.actualBytes), predicted: day.predictedBytes === null ? undefined : Number(day.predictedBytes), low: day.lowerBytes === null ? undefined : Number(day.lowerBytes), high: day.upperBytes === null ? undefined : Number(day.upperBytes) }))
  const byDay = new Map(days.map(day => [day.x, day]))
  const start = days[0]?.x, end = days.at(-1)?.x
  if (start === undefined || end === undefined) return days
  const rows = []
  for (let at = start; at <= end && rows.length < 30; at += 86400000) rows.push(byDay.get(at) ?? { x: at, label: new Date(at).toISOString().slice(0, 10), actual: undefined, predicted: undefined, low: undefined, high: undefined })
  return rows
})
const config = computed(() => ({ actual: { label: t('predictions.actual'), color: 'var(--accent)' }, predicted: { label: t('predictions.predicted'), color: 'var(--text-muted)', dashed: true } }))
const hasData = computed(() => data.value.some(day => day.actual !== undefined || day.predicted !== undefined))
</script>
<template>
  <section class="personal-predictions" :aria-busy="loading">
    <header><h3>{{ t('predictions.title') }}</h3><UButton v-if="back" color="neutral" variant="ghost" icon="i-ph-arrow-left" :aria-label="t('predictions.back')" @click="emit('back')" /></header>
    <USkeleton v-if="loading && !forecast" class="h-64 w-full" />
    <InlineNotice v-if="error" tone="warning">{{ error }}<UButton color="neutral" variant="link" :label="t('common.tryAgain')" @click="refresh()" /></InlineNotice>
    <template v-if="forecast">
      <InlineNotice v-if="forecast.stale || forecast.refreshPending" tone="warning">{{ t(forecast.stale ? 'predictions.stale' : 'predictions.pending') }}</InlineNotice>
      <LineChart v-if="hasData" :data="data" :config="config" :label="t('predictions.title')" :format-value="formatBytes" :band="{ low: 'low', high: 'high', color: 'var(--accent)' }" :bridge="{ from: 'actual', to: 'predicted', color: 'var(--text-muted)' }" connect-missing :format-x="(x: number) => formatAccountingDay(new Date(x).toISOString())" />
      <p v-else>{{ t('predictions.noHistory') }}</p>
    </template>
  </section>
</template>
<style scoped>
.personal-predictions { display: grid; gap: .75rem; min-width: 0; padding: .5rem; }.personal-predictions header { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }.personal-predictions h3, .personal-predictions p { margin: 0; }.personal-predictions p { color: var(--text-muted); font-size: .75rem; }
</style>
