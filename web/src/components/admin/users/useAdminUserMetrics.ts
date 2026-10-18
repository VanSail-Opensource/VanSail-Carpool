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
  let timer: ReturnType<typeof setInterval> | undefined
  async function load(): Promise<void> {
    if (!active) return
    controller?.abort(); controller = new AbortController()
    const ticket = ++generation
    const id = toValue(userId)
    loading.value = metrics.value === null; error.value = null
    try {
      const value = await adminOperationsApi.getUserMetrics(id, controller.signal)
      if (!active || ticket !== generation || id !== toValue(userId)) return
      metrics.value = value
      samples.append(Date.parse(value.fetchedAt), !value.nodes.some(node => node.available) ? {} : { upload: value.uploadBytesPerSecond, download: value.downloadBytesPerSecond })
    } catch (caught) { if (active && ticket === generation) { error.value = localizedError(caught, 'errors.adminLoad'); metrics.value = null; samples.clear() } }
    finally { if (active && ticket === generation) loading.value = false }
  }
  function dispose(): void { active = false; generation++; controller?.abort(); if (timer) clearInterval(timer); metrics.value = null; samples.clear() }
  watch(() => toValue(userId), () => { metrics.value = null; samples.clear(); void load() }, { immediate: true })
  onMounted(() => { timer = setInterval(() => { if (!document.hidden) void load() }, 10000); window.addEventListener('txc:session-cleared', dispose) })
  onScopeDispose(() => { dispose(); window.removeEventListener('txc:session-cleared', dispose) })
  return { metrics, loading, error, load, samples: samples.series }
}
