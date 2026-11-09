import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeApi, type XNodeEndpointField, type XNodeEndpointPreview } from '@/api/xnode'
import { createLatestRequest } from '@/utils/latestRequest'
import { localizedError } from '@/i18n'
import { endpointPayload, type EndpointDraft } from './endpointForm'

export function useEndpointPreview(draft: () => EndpointDraft, invalid: () => boolean, fields: () => XNodeEndpointField[] = () => []) {
  const result = shallowRef<XNodeEndpointPreview | null>(null), pending = shallowRef(false)
  const error = shallowRef<string | null>(null), checked = shallowRef(''), latest = createLatestRequest()
  let timer: ReturnType<typeof setTimeout> | undefined, controller: AbortController | undefined
  let context = ''
  const signature = () => JSON.stringify(endpointPayload(draft(), fields()))
  function clear() { latest.invalidate(); controller?.abort(); clearTimeout(timer); pending.value = false }
  async function load() {
    clear(); error.value = null
    if (invalid()) return
    const value = endpointPayload(draft(), fields()), key = signature(), ticket = latest.begin()
    controller = new AbortController(); pending.value = true
    try {
      const response = await xnodeApi.previewEndpoint(value, controller.signal)
      if (latest.isCurrent(ticket)) { result.value = response; checked.value = key }
    } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'endpointOptions.previewFailed') }
    finally { if (latest.isCurrent(ticket)) pending.value = false }
  }
  watch([signature, invalid], () => {
    clear(); checked.value = ''; error.value = null
    const next = `${draft().nodeId}:${draft().inboundTag}`
    if (context !== next) { context = next; result.value = null }
    if (invalid()) return
    pending.value = true; timer = setTimeout(() => void load(), 300)
  }, { immediate: true })
  onScopeDispose(() => { clear(); latest.dispose() })
  const valid = computed(() => !invalid() && !pending.value && !error.value && checked.value === signature() && result.value?.valid === true)
  return { result, pending, error, valid, load }
}
