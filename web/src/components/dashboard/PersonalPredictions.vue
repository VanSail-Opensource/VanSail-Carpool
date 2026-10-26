<script setup lang="ts">
import { computed } from 'vue'
import { forecastApi } from '@/api/forecasts'
import LineChart from '@/components/charts/LineChart.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { useI18n } from '@/i18n'
import { formatBytes, formatDate } from '@/utils/format'
const props = defineProps<{ userId?: string; back?: boolean }>()
const emit = defineEmits<{ back: [] }>()
const { t } = useI18n()
const { value: forecast, loading, error, refresh } = useSubscriptionData(signal => props.userId ? forecastApi.user(props.userId, signal) : forecastApi.personal(signal), 15_000, 'predictions.loadFailed')
const data = computed(() => (forecast.value?.days || []).map(day => ({ x: Date.parse(day.date), label: day.date, actual: day.actualBytes === null ? undefined : Number(day.actualBytes), predicted: day.predictedBytes === null ? undefined : Number(day.predictedBytes), low: day.lowerBytes === null ? undefined : Number(day.lowerBytes), high: day.upperBytes === null ? undefined : Number(day.upperBytes) })))
const config = computed(() => ({ actual: { label: t('predictions.actual'), color: 'var(--accent)' }, predicted: { label: t('predictions.predicted'), color: 'var(--text-muted)', dashed: true } }))
const hasData = computed(() => data.value.some(day => day.actual !== undefined || day.predicted !== undefined))
</script>
<template>
  <section class="personal-predictions" :aria-busy="loading">
    <header><h3>{{ t('predictions.title') }}</h3><UButton v-if="back" color="neutral" variant="ghost" icon="i-ph-arrow-left" :aria-label="t('predictions.back')" @click="emit('back')" /></header>
    <p>{{ t('predictions.period') }}</p>
    <USkeleton v-if="loading" class="h-64 w-full" />
    <InlineNotice v-else-if="error" tone="warning">{{ error }}<UButton color="neutral" variant="link" :label="t('common.tryAgain')" @click="refresh()" /></InlineNotice>
    <template v-else-if="forecast">
      <InlineNotice v-if="forecast.stale || forecast.refreshPending" tone="warning">{{ t(forecast.stale ? 'predictions.stale' : 'predictions.pending') }}</InlineNotice>
      <LineChart v-if="hasData" :data="data" :config="config" :label="t('predictions.title')" :format-value="formatBytes" :band="{ low: 'low', high: 'high', color: 'var(--accent)' }" :format-x="(x: number) => new Date(x).toISOString().slice(5, 10)" />
      <p v-else>{{ t('predictions.noHistory') }}</p>
      <dl>
        <div><dt>{{ t('predictions.interval') }}</dt><dd>{{ t('predictions.intervalValue', { value: forecast.confidence.nominalIntervalBps / 100 }) }}</dd></div>
        <div><dt>{{ t('predictions.coverage') }}</dt><dd>{{ t('predictions.coverageValue', { value: forecast.confidence.observedCoverageBps / 100, days: forecast.confidence.observedDays }) }}</dd></div>
        <div><dt>{{ t('predictions.calibration') }}</dt><dd>{{ t(forecast.confidence.calibration === 'VALIDATED' ? 'predictions.validated' : 'predictions.unvalidated') }}<small v-if="forecast.confidence.evaluatedIntervalCoverage !== null">{{ t('predictions.calibrated', { value: (forecast.confidence.evaluatedIntervalCoverage * 100).toFixed(1), samples: forecast.confidence.samples }) }}</small></dd></div>
        <div><dt>{{ t('predictions.freshness') }}</dt><dd>{{ forecast.generatedAt ? formatDate(forecast.generatedAt) : t('common.notAvailable') }}</dd></div>
      </dl>
      <p class="personal-predictions__note">{{ t('predictions.confidenceHint') }}</p>
    </template>
  </section>
</template>
<style scoped>
.personal-predictions { display: grid; gap: .75rem; min-width: 0; padding: .5rem; }.personal-predictions header { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }.personal-predictions h3, .personal-predictions p, .personal-predictions dl { margin: 0; }.personal-predictions p, .personal-predictions dt { color: var(--text-muted); font-size: .75rem; }.personal-predictions dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .75rem; }.personal-predictions dd { margin: .2rem 0 0; font-size: .8rem; overflow-wrap: anywhere; }.personal-predictions dd small { display: block; margin-top: .25rem; } .personal-predictions__note { line-height: 1.5; }
@media(max-width: 359px) { .personal-predictions dl { grid-template-columns: 1fr; } }
</style>
