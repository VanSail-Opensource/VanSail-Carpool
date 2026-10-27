import { onScopeDispose, shallowRef, watch, type MaybeRefOrGetter, toValue } from 'vue'
import { pricingAnalysisApi, type PricingRecord } from '@/api/pricingAnalysis'
import { localizedError } from '@/i18n'
export function usePricingDataset(name: string, product: MaybeRefOrGetter<string | undefined>, user: MaybeRefOrGetter<string | undefined>) {
 const items = shallowRef<PricingRecord[]>([])
 const loading = shallowRef(false)
 const error = shallowRef('')
 const nextCursor = shallowRef('')
 const truncated = shallowRef(false)
 let controller: AbortController | undefined
 let generation = 0
 async function load(more = false) {
  controller?.abort()
  const version = ++generation
  const abort = new AbortController(); controller = abort
  loading.value = true; error.value = ''
  try {
   const page = await pricingAnalysisApi.data(name, { product: toValue(product) || undefined, user: toValue(user) || undefined, cursor: more ? nextCursor.value : undefined }, abort.signal)
   if (abort.signal.aborted || version !== generation) return
   items.value = more ? [...items.value, ...page.items] : page.items
   nextCursor.value = page.nextCursor ?? ''; truncated.value = page.truncated
  } catch (caught) { if (!abort.signal.aborted && version === generation) error.value = localizedError(caught, 'pricing.loadError') }
  finally { if (version === generation) loading.value = false }
 }
 watch(() => [toValue(product), toValue(user)], () => { items.value = []; nextCursor.value = ''; void load() }, { immediate: true })
 onScopeDispose(() => { generation++; controller?.abort() })
 return { items, loading, error, nextCursor, truncated, load }
}
