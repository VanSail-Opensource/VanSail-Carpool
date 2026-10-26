import { onScopeDispose, shallowRef, watch, type Ref } from 'vue'
import { forecastApi, type UserPricePage } from '@/api/forecasts'
import { localizedError } from '@/i18n'
export function useUserPrices(userId: Ref<string>) {
 const items = shallowRef<UserPricePage['items']>([])
 const cursor = shallowRef('')
 const loading = shallowRef(false)
 const error = shallowRef<string | null>(null)
 let controller: AbortController | undefined
 let generation = 0
 async function load(more = false): Promise<void> {
  const version = ++generation
  controller?.abort(); controller = new AbortController()
  const signal = controller.signal
  loading.value = true; error.value = null
  if (!more) { items.value = []; cursor.value = '' }
  try {
   const page = await forecastApi.prices(userId.value, more ? cursor.value : undefined, signal)
   if (version !== generation) return
   items.value = more ? [...items.value, ...page.items] : page.items
   cursor.value = page.nextCursor
  } catch (caught) { if (!signal.aborted && version === generation) error.value = localizedError(caught, 'pricing.loadError') }
  finally { if (version === generation) loading.value = false }
 }
 watch(userId, () => void load(), { immediate: true })
 onScopeDispose(() => { generation++; controller?.abort() })
 return { items, cursor, loading, error, load }
}
