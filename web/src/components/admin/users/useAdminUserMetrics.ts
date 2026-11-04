import { onMounted, onScopeDispose, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { adminOperationsApi, type XNodeProfileMetrics } from '@/api/adminOperations'
import { localizedError } from '@/i18n'
import { useMetricSamples } from '@/composables/useMetricSamples'

export function useAdminUserMetrics(userId: MaybeRefOrGetter<string>) {
  const metrics = shallowRef<XNodeProfileMetrics | null>(null)
  const loading = shallowRef(true)
  const error = shallowRef<string | null>(null)
  const samples = useMetricSamples()
  let controller: AbortController | undefined
  let generation = 0
  let active = true
  let timer: ReturnType<typeof setTimeout> | undefined
  async function load(): Promise<void> {
    if (!active) return
    clearTimeout(timer)
    controller?.abort(); controller = new AbortController()
    const ticket = ++generation
    const id = toValue(userId)
    loading.value = true
    try {
      const value = await adminOperationsApi.getUserMetrics(id, controller.signal)
      if (!active || ticket !== generation || id !== toValue(userId)) return
      metrics.value = value
      error.value = null
      samples.append(Date.parse(value.fetchedAt), !value.nodes.some(node => node.available) ? {} : { upload: value.uploadBytesPerSecond, download: value.downloadBytesPerSecond })
    } catch (caught) { if (active && ticket === generation) { error.value = localizedError(caught, 'errors.adminLoad'); samples.clear() } }
    finally {
      if (active && ticket === generation) {
        loading.value = false
        if (!document.hidden) timer = setTimeout(() => void load(), 10000)
      }
    }
  }
  function visibility(): void { clearTimeout(timer); if (!document.hidden) void load() }
  function dispose(): void { active = false; generation++; controller?.abort(); clearTimeout(timer); metrics.value = null; samples.clear() }
  watch(() => toValue(userId), () => { metrics.value = null; error.value = null; samples.clear(); void load() }, { immediate: true })
  onMounted(() => { window.addEventListener('txc:session-cleared', dispose); document.addEventListener('visibilitychange', visibility) })
  onScopeDispose(() => { dispose(); window.removeEventListener('txc:session-cleared', dispose); document.removeEventListener('visibilitychange', visibility) })
  return { metrics, loading, error, load, samples: samples.series }
}
