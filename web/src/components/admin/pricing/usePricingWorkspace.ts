import { computed, onScopeDispose, shallowRef, watch, type MaybeRefOrGetter, toValue } from 'vue'
import { pricingAnalysisApi, type PricingDashboard, type PricingHealth } from '@/api/pricingAnalysis'
import { t } from '@/i18n'

export function usePricingWorkspace(_product: MaybeRefOrGetter<string>, _user: MaybeRefOrGetter<string>, mode: MaybeRefOrGetter<string>) {
 const dashboard = shallowRef<PricingDashboard | null>(null)
 const health = shallowRef<PricingHealth[]>([])
 const loading = shallowRef(false)
 const error = shallowRef('')
 const cursor = shallowRef('')
 let generation = 0
 let dashboardAt = 0
 let controller: AbortController | undefined
 let poll: AbortController | undefined
 let timer: ReturnType<typeof setTimeout> | undefined
 const live = () => ['overview', 'operations'].includes(toValue(mode))
 function schedule() { if (timer) clearTimeout(timer); if (!document.hidden && live()) timer = setTimeout(() => void refreshHealth(), 20_000) }
 async function refreshHealth() {
  poll?.abort()
  if (document.hidden || !live()) return
  if (toValue(mode) === 'overview' && Date.now() - dashboardAt >= 60_000) { await load(); return }
  const abort = new AbortController(); poll = abort
  const version = generation
  try { const result = await pricingAnalysisApi.health(abort.signal); if (!abort.signal.aborted && version === generation) { health.value = result.health; error.value = '' } }
  catch { if (!abort.signal.aborted && version === generation) error.value = t('pricing.loadError') }
  finally { if (version === generation && !abort.signal.aborted) schedule() }
 }
 async function load() {
  const version = ++generation
  controller?.abort(); poll?.abort()
  if (timer) clearTimeout(timer)
  const abort = new AbortController(); controller = abort
  error.value = ''
  if (!live()) { loading.value = false; return }
  loading.value = true
  try {
   if (toValue(mode) === 'overview') {
    const result = await pricingAnalysisApi.dashboard(cursor.value, abort.signal)
    if (!abort.signal.aborted && version === generation) { dashboard.value = result; health.value = result.health; dashboardAt = Date.now() }
   } else {
    const result = await pricingAnalysisApi.health(abort.signal)
    if (!abort.signal.aborted && version === generation) health.value = result.health
   }
  } catch { if (!abort.signal.aborted && version === generation) error.value = t('pricing.loadError') }
  finally { if (!abort.signal.aborted && version === generation) { loading.value = false; schedule() } }
 }
 watch(() => [toValue(mode), cursor.value], () => { dashboard.value = null; health.value = []; void load() }, { immediate: true })
 function visibility() { if (document.hidden) { if (timer) clearTimeout(timer); poll?.abort() } else if (toValue(mode) === 'overview') void load(); else void refreshHealth() }
 document.addEventListener('visibilitychange', visibility)
 onScopeDispose(() => { generation++; controller?.abort(); poll?.abort(); if (timer) clearTimeout(timer); document.removeEventListener('visibilitychange', visibility) })
 return { dashboard, health, loading, error, cursor, load, nextCursor: computed(() => dashboard.value?.nextCursor ?? '') }
}
