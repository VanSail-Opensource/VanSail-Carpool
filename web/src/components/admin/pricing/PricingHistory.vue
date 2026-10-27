<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { pricingAnalysisApi, type PricingForecast, type PricingRecord, type PricingValidation } from '@/api/pricingAnalysis'
import MetricLineChart from '@/components/common/MetricLineChart.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import { formatBytes, formatDateTime, formatAccountingDay } from '@/utils/format'

const props = defineProps<{ entityType: string; entityId: string; events: PricingRecord[]; providerCycle?: PricingRecord }>()
const { t } = useI18n()
const horizon = shallowRef(86400)
const alignment = shallowRef('calendar')
const model = shallowRef(props.entityType === 'product' ? 'empirical-receipts-v1' : props.entityType === 'user' ? 'empirical-user-total-v1' : 'empirical-daily-v1')
const from = shallowRef(new Date(Date.now() - 30 * 86400000).toISOString().slice(0, 10))
const until = shallowRef(new Date(Date.now() + 8 * 86400000).toISOString().slice(0, 10))
watch(alignment, value => {
  if (value !== 'provider' || !props.providerCycle) return
  const start = Date.parse(String(props.providerCycle.effective_at)); const cycle = Number(props.providerCycle.cycle_days) * 86400000
  if (!Number.isFinite(start) || cycle <= 0) return
  const cycleStart = start + Math.max(0, Math.floor((Date.now() - start) / cycle)) * cycle
  from.value = new Date(cycleStart).toISOString().slice(0, 10); until.value = new Date(cycleStart + cycle).toISOString().slice(0, 10)
})
const actual = shallowRef(true)
const predicted = shallowRef(true)
const band = shallowRef(true)
const loading = shallowRef(false)
const error = shallowRef('')
const forecasts = shallowRef<PricingForecast[]>([])
const metrics = shallowRef<PricingValidation | null>(null)
const metric = computed(() => props.entityType === 'product' ? 'gross_revenue_minor' : 'traffic_bytes')
const historyKey = computed(() => [props.entityType, props.entityId, horizon.value, model.value, from.value, until.value].join('|'))
watch(historyKey, async (_, __, onCleanup) => {
  const abort = new globalThis.AbortController()
  onCleanup(() => abort.abort())
  forecasts.value = []; metrics.value = null; error.value = ''; loading.value = false
  if (!props.entityId) return
  if (!from.value || !until.value || from.value >= until.value) { error.value = t('pricing.invalidDates'); return }
  loading.value = true
  forecasts.value = []
  metrics.value = null
  error.value = ''
  try {
    const response = await pricingAnalysisApi.history({ entityType: props.entityType, entityId: props.entityId, metric: metric.value, modelVersion: model.value, horizon: horizon.value, from: `${from.value}T00:00:00Z`, until: `${until.value}T00:00:00Z` }, abort.signal)
    if (abort.signal.aborted) return
    forecasts.value = [...response.items].reverse()
    metrics.value = response.metrics
  } catch { if (!abort.signal.aborted) error.value = t('pricing.loadError') }
  finally { if (!abort.signal.aborted) loading.value = false }
}, { immediate: true })
const calendar = computed(() => {
  const existing = new Map(forecasts.value.map(f => [Date.parse(f.targetFrom), f]))
  const start = Date.parse(forecasts.value[0]?.targetFrom ?? '')
  const end = Date.parse(forecasts.value.at(-1)?.targetFrom ?? '')
  const rows = []
  for (let at = start; at <= end && rows.length < 500; at += 86400000) rows.push({ at, forecast: existing.get(at) })
  return rows
})
const samples = computed(() => actual.value ? calendar.value.map(({ at, forecast: f }) => ({ at, value: f?.status === 'COMPLETE' && f.actual !== null ? f.actual : Number.NaN })) : [])
const predictions = computed(() => predicted.value ? calendar.value.map(({ at, forecast: f }) => ({ at, value: f?.predictedP50 ?? Number.NaN, low: f?.predictedP10 ?? Number.NaN, high: f?.predictedP90 ?? Number.NaN })) : [])
const events = computed(() => props.events.map(event => ({ at: Date.parse(String(event.started_at ?? event.created_at ?? event.watermark ?? '')), label: `${event.event_type ? t(`pricing.events.${event.event_type}`) : t(`pricing.states.${event.state ?? event.status}`)} · ${String(event.reason ?? '')}` })).filter(event => Number.isFinite(event.at)))
const columns = computed(() => [{ id: 'date', header: t('pricing.date') }, { id: 'prediction', header: t('pricing.prediction') }, { id: 'actual', header: t('pricing.actual') }, { id: 'interval', header: t('pricing.interval') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'coverage', header: t('pricing.completeness') }])
function display(value: number): string { return metric.value === 'traffic_bytes' ? `${value < 0 ? '−' : ''}${formatBytes(String(Math.round(Math.abs(value))))}` : (value / 100).toFixed(2) }
</script>
<template>
  <section class="pricing-history">
    <h3>{{ t('pricing.history') }}</h3>
    <p class="text-muted">{{ t('pricing.originalForecasts') }}</p>
    <UFormField v-if="providerCycle" :label="t('pricing.alignment')"><USelect v-model="alignment" :items="[{ value: 'calendar', label: t('pricing.calendar') }, { value: 'provider', label: t('pricing.providerCycle') }]" class="w-full max-w-sm" /></UFormField>
    <div class="pricing-history__filters">
      <UFormField :label="t('pricing.horizon')"><USelect v-model="horizon" :items="[1, 2, 3, 4, 5, 7].map(day => ({ value: day * 86400, label: t('pricing.leadDays', { count: day }) }))" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.modelVersion')"><UInput v-model="model" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.from')"><UInput v-model="from" type="date" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.until')"><UInput v-model="until" type="date" class="w-full" /></UFormField>
    </div>
    <div class="pricing-history__toggles"><UCheckbox v-model="actual" :label="t('pricing.actualSolid')" /><UCheckbox v-model="predicted" :label="t('pricing.predictedDashed')" /><UCheckbox v-model="band" :label="t('pricing.interval')" /></div>
    <p v-if="loading" role="status">{{ t('common.loading') }}</p>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <template v-else-if="forecasts.length">
      <MetricLineChart :samples="samples" :forecasts="predictions" :show-band="band" :events="events" :label="t('pricing.history')" :unit="metric === 'traffic_bytes' ? 'bytes' : 'money'" connect-missing />
      <dl v-if="metrics?.samples" class="pricing-history__metrics">
        <div><dt>{{ t('pricing.mae') }}</dt><dd>{{ display(metrics.mae) }}</dd></div>
        <div><dt>{{ t('pricing.wape') }}</dt><dd>{{ metrics.wape === null ? t('pricing.undefined') : `${(metrics.wape * 100).toFixed(1)}%` }}</dd></div>
        <div><dt>{{ t('pricing.bias') }}</dt><dd>{{ display(metrics.bias) }}</dd></div>
        <div><dt>{{ t('pricing.coverage') }}</dt><dd>{{ (metrics.coverage * 100).toFixed(1) }}%</dd></div>
      </dl>
      <UTable :data="forecasts" :columns="columns" class="admin-table">
        <template #date-cell="{ row }"><time :datetime="row.original.targetFrom">{{ formatAccountingDay(row.original.targetFrom) }}</time><small class="block text-muted">{{ t('pricing.issuedAt', { date: formatDateTime(row.original.generatedAt) }) }}</small></template>
        <template #prediction-cell="{ row }">{{ display(row.original.predictedP50) }}</template>
        <template #actual-cell="{ row }">{{ row.original.actual === null ? t('common.notAvailable') : display(row.original.actual) }}</template>
        <template #interval-cell="{ row }">{{ display(row.original.predictedP10) }} – {{ display(row.original.predictedP90) }}</template>
        <template #status-cell="{ row }">{{ t(`pricing.states.${row.original.status}`) }}<small class="block text-muted">{{ t(`pricing.states.${row.original.calibrationStatus}`) }}</small></template>
        <template #coverage-cell="{ row }">{{ (row.original.dataCoverageBps / 100).toFixed(0) }}%</template>
      </UTable>
    </template>
    <p v-else-if="!loading">{{ t('pricing.noForecasts') }}</p>
    <UButton variant="link" :to="{ path: '/admin/pricing/exports', query: { from, until, nodes: entityType === 'node' ? entityId : undefined, users: entityType === 'user_node' ? entityId.split(':')[0] : undefined, products: entityType === 'product' ? entityId : undefined } }" :label="t('pricing.exportDisplayed')" />
  </section>
</template>
<style scoped>
.pricing-history { display: grid; gap: 1rem; min-width: 0; }
.pricing-history h3, .pricing-history p { margin: 0; }
.pricing-history h3 { font-size: 1rem; }
.pricing-history__filters { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: .75rem; }
.pricing-history__toggles { display: flex; flex-wrap: wrap; gap: 1rem; }
.pricing-history__metrics { display: flex; flex-wrap: wrap; gap: 1rem 2rem; margin: 0; font-size: .8rem; }
.pricing-history__metrics dt { color: var(--text-muted); }.pricing-history__metrics dd { margin: .25rem 0 0; font-variant-numeric: tabular-nums; }
@media (max-width: 700px) { .pricing-history__filters { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 380px) { .pricing-history__filters { grid-template-columns: 1fr; } }
</style>
