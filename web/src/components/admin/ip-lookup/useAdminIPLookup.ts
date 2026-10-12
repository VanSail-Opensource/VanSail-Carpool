import { onScopeDispose, reactive, shallowRef, watch } from 'vue'
import { api } from '@/api/client'
import { ApiError } from '@/api/http'
import { ipLookupApi, type IPLookupAdminSettings } from '@/api/ipLookup'
import type { Combo } from '@/api/types'
import { useSessionStore } from '@/stores/session'
import { useIPLookupAvailability } from '@/composables/useIPLookupAvailability'

// Stored settings predating the retry policy decode without it; the enabled
// server defaults apply until an administrator saves explicit values.
const defaultRetry = () => ({ enabled: true, maxAttempts: 3, delaySeconds: 60 })

function normalizeRetry(settings: IPLookupAdminSettings): void {
  if (!settings.retry) settings.retry = defaultRetry()
  settings.refreshQuotaCost ??= 0
  settings.proxySources ??= []
  settings.dnsResolvers ??= ['1.1.1.1:53', '8.8.8.8:53', '9.9.9.9:53']
  settings.probePorts ??= [443, 4430, 8443]
}

export function useAdminIPLookup() {
  const session = useSessionStore()
  const availability = useIPLookupAvailability()
  const settings = reactive<IPLookupAdminSettings>({ enabled: false, lookupFeeTxb: '', refreshFeeTxb: '', providers: [], geolocationOrder: ['ip2location', 'ipapi', 'maxmind', 'scamalytics', 'abuseipdb'], retry: defaultRetry(), credentials: {}, configured: {}, comboQuotas: {} })
  const savedSourceConfig = shallowRef('')
  const combos = shallowRef<Combo[]>([])
  const loading = shallowRef(true)
  const busy = shallowRef(false)
  const errorCode = shallowRef('')
  const saved = shallowRef(false)
  let disposed = false
  let version = 0
  const setError = (error: unknown) => { errorCode.value = error instanceof ApiError ? error.code : 'IP_LOOKUP_FAILED' }

  async function load(): Promise<void> {
    const current = ++version
    const owner = session.user?.id
    try {
      const [value, catalog] = await Promise.all([ipLookupApi.settings(), api.getAdminResource<{ items: Combo[] }>('combos')])
      if (disposed || current !== version || owner !== session.user?.id) return
      Object.assign(settings, value)
      normalizeRetry(settings)
      savedSourceConfig.value = JSON.stringify({ sources: settings.proxySources, resolvers: settings.dnsResolvers })
      settings.credentials = Object.fromEntries([...value.providers.map(provider => provider.id), 'cloudflare_radar'].map(id => [id, { value: '', clear: false }]))
      combos.value = catalog.items
      errorCode.value = ''
    } catch (error) { if (!disposed && current === version) setError(error) }
    finally { if (!disposed && current === version) loading.value = false }
  }

  async function save(): Promise<void> {
    if (busy.value) return
    busy.value = true
    saved.value = false
    errorCode.value = ''
    const current = version
    try {
      const value = await ipLookupApi.saveSettings(settings)
      if (disposed || current !== version) return
      Object.assign(settings, value)
      normalizeRetry(settings)
      savedSourceConfig.value = JSON.stringify({ sources: settings.proxySources, resolvers: settings.dnsResolvers })
      settings.credentials = Object.fromEntries([...value.providers.map(provider => provider.id), 'cloudflare_radar'].map(id => [id, { value: '', clear: false }]))
      saved.value = true
      await availability.refresh()
    } catch (error) { if (!disposed && current === version) setError(error) }
    finally { if (!disposed && current === version) busy.value = false }
  }

  watch(() => session.user?.id, () => { saved.value = false; busy.value = false; errorCode.value = ''; loading.value = true; void load() }, { immediate: true })
  onScopeDispose(() => { disposed = true; version++ })
  return { settings, savedSourceConfig, combos, loading, busy, errorCode, saved, save, load }
}
