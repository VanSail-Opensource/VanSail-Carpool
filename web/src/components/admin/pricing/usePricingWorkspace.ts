import { computed, onScopeDispose, shallowRef, watch, type MaybeRefOrGetter, toValue } from 'vue'
import { pricingAnalysisApi, type PricingDashboard, type PricingRecord } from '@/api/pricingAnalysis'
import { t } from '@/i18n'

export function usePricingWorkspace(product: MaybeRefOrGetter<string>, user: MaybeRefOrGetter<string>, mode: MaybeRefOrGetter<string>) {
  const dashboard = shallowRef<PricingDashboard | null>(null)
  const data = shallowRef<Record<string, PricingRecord[]>>({})
  const loading = shallowRef(true)
  const error = shallowRef('')
  const cursor = shallowRef('')
  let controller: AbortController | null = null
  async function load(): Promise<void> {
    controller?.abort()
    if (toValue(mode) === 'exports') { loading.value = false; return }
    const abort = new AbortController()
    controller = abort
    loading.value = true
    error.value = ''
    try {
      dashboard.value = await pricingAnalysisApi.dashboard(cursor.value, abort.signal)
      const query = { product: toValue(product).split(':').slice(1).join(':') || undefined, user: toValue(user) || undefined }
      const datasets = ['pricing/published_prices', 'pricing/pricing_decisions', 'forecasts/renewal_predictions', 'forecasts/model_versions', 'operations/admin_overrides', 'operations/health_incidents', 'validation/model_metrics', 'features/features', 'pricing/provider_costs', 'pricing/contracts', 'validation/calibration', 'operations/policy_events', 'pricing/provider_cost_versions']
      if (toValue(user)) datasets.push('pricing/eligible_offers')
      const entries: [string, PricingRecord[]][] = []
      for (let offset = 0; offset < datasets.length; offset += 3) {
        const group = await Promise.all(datasets.slice(offset, offset + 3).map(async name => {
          const response = await pricingAnalysisApi.data(name, query, abort.signal)
          return [name, response.items] as [string, PricingRecord[]]
        }))
        entries.push(...group)
      }
      data.value = Object.fromEntries(entries)
    } catch { if (!abort.signal.aborted) error.value = t('pricing.loadError') }
    finally { if (!abort.signal.aborted) loading.value = false }
  }
  watch(() => [toValue(product), toValue(user), toValue(mode), cursor.value], () => { dashboard.value = null; data.value = {}; void load() }, { immediate: true })
  const timer = window.setInterval(() => { if (toValue(mode) !== 'exports' && !document.hidden && !loading.value) void pricingAnalysisApi.dashboard(cursor.value).then(value => { dashboard.value = value }).catch(() => { error.value = t('pricing.loadError') }) }, 20000)
  onScopeDispose(() => { controller?.abort(); window.clearInterval(timer) })
  return { dashboard, data, loading, error, cursor, load, nextCursor: computed(() => dashboard.value?.nextCursor ?? '') }
}
