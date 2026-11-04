import { computed, onMounted, onScopeDispose, ref, shallowRef } from 'vue'
import { xnodeApi, type XNodeGlobal } from '@/api/xnode'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useXNodeGlobalSettings(part?: 'automaticWhitelist') {
  const draft = ref<XNodeGlobal | null>(null)
  const loading = shallowRef(true)
  const busy = shallowRef(false)
  const loaded = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  let controller: AbortController | undefined
  const canSave = computed(() => loaded.value && !loading.value && !busy.value)

  async function load() {
    if (busy.value) return
    controller?.abort()
    controller = new AbortController()
    const ticket = latest.begin()
    loading.value = true; loaded.value = false; error.value = null
    try {
      const value = await xnodeApi.global(controller.signal)
      if (!latest.isCurrent(ticket)) return
      draft.value = { ...value, automaticWhitelist: value.automaticWhitelist ?? { enabled: true, durationHours: 24 }, telemetry: { ...value.telemetry, include_active_ips: true }, sourceGeoIp: value.sourceGeoIp ?? { enabled: false, selectors: ['geoip:cn'] }, blockedDomains: value.blockedDomains ?? [] }
      loaded.value = true
    } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'xnode.loadFailed') }
    finally { if (latest.isCurrent(ticket)) loading.value = false }
  }
  async function save() {
    if (!draft.value || !canSave.value) return
    const ticket = latest.begin()
    busy.value = true; error.value = null
    try {
      const body = { ...draft.value }
      // The general form does not edit leases. Omission preserves newer lease
      // changes; the dedicated form uses a server-side locked field merge.
      if (!part) delete body.automaticWhitelist
      const value = await xnodeApi.saveGlobal(body, part)
      if (latest.isCurrent(ticket)) draft.value = value
    }
    catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'xnode.saveFailed') }
    finally { if (latest.isCurrent(ticket)) busy.value = false }
  }
  onMounted(load)
  onScopeDispose(() => { controller?.abort(); latest.dispose() })
  return { draft, loading, busy, error, canSave, load, save }
}
