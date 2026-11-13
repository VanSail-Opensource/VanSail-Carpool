import { onScopeDispose, reactive, shallowRef, watch } from 'vue'
import { api } from '@/api/client'
import { ApiError } from '@/api/http'
import { ipLookupApi, type IPLookupAdminSettings } from '@/api/ipLookup'
import type { Combo } from '@/api/types'
import { useSessionStore } from '@/stores/session'

// Stored settings predating the retry policy decode without it; the enabled
// server defaults apply until an administrator saves explicit values.
const defaultRetry = () => ({ enabled: true, maxAttempts: 3, delaySeconds: 60 })
const emptySettings = (): IPLookupAdminSettings => ({ enabled: false, lookupFeeTxb: '', refreshFeeTxb: '', providers: [], geolocationOrder: ['ip2location', 'ipapi', 'maxmind', 'scamalytics', 'abuseipdb'], retry: defaultRetry(), credentials: {}, configured: {}, comboQuotas: {} })

function normalizeRetry(settings: IPLookupAdminSettings): void {
  if (!settings.retry) settings.retry = defaultRetry()
  settings.refreshQuotaCost ??= 0
}

export function useAdminIPLookup() {
  const session = useSessionStore()
  const settings = reactive<IPLookupAdminSettings>(emptySettings())
  const combos = shallowRef<Combo[]>([])
  const loading = shallowRef(true)
  const loaded = shallowRef(false)
  const busy = shallowRef(false)
  const errorCode = shallowRef('')
  const saved = shallowRef(false)
  let disposed = false
  let version = 0
  const setError = (error: unknown) => { errorCode.value = error instanceof ApiError ? error.code : 'IP_LOOKUP_FAILED' }

  async function load(): Promise<void> {
    if (disposed || busy.value) return
    const current = ++version
    const owner = session.user?.id
    loading.value = true; loaded.value = false
    saved.value = false; errorCode.value = ''
    try {
      const [value, catalog] = await Promise.all([ipLookupApi.settings(), api.getAdminResource<{ items: Combo[] }>('combos')])
      if (disposed || current !== version || owner !== session.user?.id) return
      Object.assign(settings, value)
      normalizeRetry(settings)
      settings.credentials = Object.fromEntries([...value.providers.map(provider => provider.id), 'cloudflare_radar'].map(id => [id, { value: '', clear: false }]))
      combos.value = catalog.items
      loaded.value = true
      errorCode.value = ''
    } catch (error) { if (!disposed && current === version && owner === session.user?.id) setError(error) }
    finally { if (!disposed && current === version && owner === session.user?.id) loading.value = false }
  }

  async function save(): Promise<void> {
    if (disposed || busy.value || loading.value || !loaded.value) return
    busy.value = true
    saved.value = false
    errorCode.value = ''
    const current = ++version
    const owner = session.user?.id
    const input: IPLookupAdminSettings = JSON.parse(JSON.stringify(settings))
    try {
      const value = await ipLookupApi.saveSettings(input)
      if (disposed || current !== version || owner !== session.user?.id) return
      Object.assign(settings, value)
      normalizeRetry(settings)
      settings.credentials = Object.fromEntries([...value.providers.map(provider => provider.id), 'cloudflare_radar'].map(id => [id, { value: '', clear: false }]))
      saved.value = true
    } catch (error) { if (!disposed && current === version && owner === session.user?.id) setError(error) }
    finally { if (!disposed && current === version && owner === session.user?.id) busy.value = false }
  }

  function clear(): void {
    Object.assign(settings, emptySettings())
    combos.value = []
    loaded.value = false
    saved.value = false
    busy.value = false
    errorCode.value = ''
  }
  watch(() => session.user?.id, () => { clear(); void load() }, { immediate: true })
  onScopeDispose(() => { disposed = true; version++; clear() })
  return { settings, combos, loading, loaded, busy, errorCode, saved, save, load }
}
