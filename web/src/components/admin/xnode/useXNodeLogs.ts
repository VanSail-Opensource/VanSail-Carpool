import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeApi } from '@/api/xnode'
import type { components } from '@/api/generated'
import { localizedError } from '@/i18n'

export function useXNodeLogs(nodeId: () => string) {
  const snapshot = shallowRef<components['schemas']['XNodeLogs'] | null>(null)
  const loading = shallowRef(false), paused = shallowRef(false)
  const error = shallowRef<string | null>(null)
  let timer: ReturnType<typeof setTimeout> | undefined
  let controller: AbortController | undefined
  let generation = 0, disposed = false
  const text = computed(() => (snapshot.value?.entries ?? []).map(entry => {
    const string = (key: string) => typeof entry[key] === 'string' ? entry[key] as string : ''
    return [string('time'), string('source'), string('text')].filter(Boolean).join(' ')
  }).filter(Boolean).join('\n'))
  function stop() { if (timer) clearTimeout(timer); controller?.abort(); generation++ }
  function schedule() {
    timer = setTimeout(() => { if (document.visibilityState === 'visible') void refresh(); else schedule() }, 2000)
  }
  async function refresh() {
    stop()
    const id = nodeId(), ticket = generation
    if (!id || disposed) return
    controller = new AbortController(); loading.value = true; error.value = null
    try {
      const value = await xnodeApi.logs(id, controller.signal)
      if (!disposed && ticket === generation && id === nodeId()) snapshot.value = value
    } catch (caught) { if (!disposed && ticket === generation) error.value = localizedError(caught, 'xnode.logsFailed') }
    finally {
      if (!disposed && ticket === generation) {
        loading.value = false
        if (!paused.value && nodeId()) schedule()
      }
    }
  }
  function retire() { stop(); snapshot.value = null; loading.value = false; paused.value = true }
  watch(nodeId, () => { stop(); snapshot.value = null; error.value = null; paused.value = false; void refresh() }, { immediate: true })
  watch(paused, value => { if (value) { stop(); loading.value = false } else if (nodeId()) void refresh() })
  window.addEventListener('txc:session-cleared', retire)
  window.addEventListener('txc:browser-session-expired', retire)
  onScopeDispose(() => { disposed = true; retire(); window.removeEventListener('txc:session-cleared', retire); window.removeEventListener('txc:browser-session-expired', retire) })
  return { snapshot, text, loading, paused, error, refresh }
}
