<script setup lang="ts">
import { onScopeDispose, shallowRef, watch } from 'vue'
import { createLatestRequest } from '@/utils/latestRequest'
import type { LuckyDrawForecast } from '@/api/features'
import { featuresApi } from '@/api/features'
import { useI18n } from '@/i18n'
import { txbInputFromMinor } from '@/utils/format'

const latest = createLatestRequest()
const props = defineProps<{ drawId: string }>()
const { t } = useI18n()
const forecast = shallowRef<LuckyDrawForecast | null>(null)
const loading = shallowRef(true)
const error = shallowRef(false)
async function load(): Promise<void> {
  const ticket = latest.begin()
  loading.value = true
  forecast.value = null
  error.value = false
  try {
    const value = await featuresApi.getAdminLuckyDrawForecast(props.drawId)
    if (latest.isCurrent(ticket)) forecast.value = value
  } catch { if (latest.isCurrent(ticket)) error.value = true }
  finally { if (latest.isCurrent(ticket)) loading.value = false }
}
function money(value: string | null | undefined): string {
  return value == null ? '—' : txbInputFromMinor(value) + ' TXB'
}
function range(low: string | null | undefined, high: string | null | undefined): string {
  return low == null || high == null ? '—' : money(low) + ' – ' + money(high)
}
watch(() => props.drawId, () => void load(), { immediate: true })
onScopeDispose(latest.dispose)
</script>

<template>
  <section class="draw-forecast" aria-live="polite">
    <h4>{{ t('adminDrawRedesign.forecast') }}</h4>
    <USkeleton v-if="loading" class="h-24 w-full" />
    <template v-else-if="error">
      <UAlert color="warning" variant="soft" icon="i-ph-warning" :description="t('adminDrawRedesign.forecastError')" />
      <UButton color="neutral" variant="outline" :label="t('common.tryAgain')" @click="load" />
    </template>
    <template v-else-if="forecast">
      <dl class="draw-forecast__numbers">
        <div><dt>{{ t('adminDrawRedesign.income') }}</dt><dd>{{ money(forecast.incomeMinor) }}</dd></div>
        <div><dt>{{ t('adminDrawRedesign.expectedExpense') }}</dt><dd>{{ range(forecast.expectedExpenseMinMinor, forecast.expectedExpenseMaxMinor) }}</dd></div>
        <div><dt>{{ t('adminDrawRedesign.possibleExpense') }}</dt><dd>{{ range(forecast.possibleExpenseMinMinor, forecast.possibleExpenseMaxMinor) }}</dd></div>
        <div><dt>{{ t('adminDrawRedesign.breakEven') }}</dt><dd>{{ range(forecast.breakEvenMinMinor, forecast.breakEvenMaxMinor) }}</dd></div>
        <div><dt>{{ t('adminDrawRedesign.averageBalance') }}</dt><dd>{{ money(forecast.averageBalanceMinor) }}</dd></div>
      </dl>
      <p v-if="forecast.multiplierUnavailable" class="draw-forecast__note">{{ t('adminDrawRedesign.noAverage') }}</p>
      <p v-if="forecast.couponOneTermOnly" class="draw-forecast__note">{{ t('adminDrawRedesign.couponNote') }}</p>
    </template>
  </section>
</template>

<style scoped>
.draw-forecast { display: grid; gap: 0.8rem; padding-top: 1rem; border-top: 1px solid var(--line); }
.draw-forecast h4, .draw-forecast__note { margin: 0; }
.draw-forecast__numbers { display: grid; gap: 0.15rem; margin: 0; }
.draw-forecast__numbers > div { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 0.5rem; padding: 0.55rem 0; border-bottom: 1px solid var(--line); }
.draw-forecast dt { color: var(--text-muted); }
.draw-forecast dd { margin: 0; font-weight: 600; }
.draw-forecast__note { color: var(--text-muted); font-size: 0.78rem; }
</style>
