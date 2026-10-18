import { onScopeDispose, shallowRef } from 'vue'

export interface MetricSample { at: number; value: number }
export const metricSampleLimit = 300

// Each mounted consumer owns only its displayed series; no persistent cache.
export function useMetricSamples() {
  const series = shallowRef<Record<string, MetricSample[]>>({})
  let retired = false
  function clear(): void { series.value = {} }
  function retire(): void { retired = true; clear() }
  function append(at: number, values: Record<string, number>): void {
    if (retired || !Number.isFinite(at)) return
    const next: Record<string, MetricSample[]> = {}
    for (const [key, value] of Object.entries(values)) {
      if (!Number.isFinite(value) || value < 0) continue
      const previous = series.value[key] ?? []
      const last = previous.at(-1)
      next[key] = last && last.at >= at ? previous : [...previous.slice(-(metricSampleLimit - 1)), { at, value }]
    }
    series.value = next
  }
  window.addEventListener('txc:session-cleared', retire)
  window.addEventListener('txc:browser-session-expired', retire)
  onScopeDispose(() => {
    retire()
    window.removeEventListener('txc:session-cleared', retire)
    window.removeEventListener('txc:browser-session-expired', retire)
  })
  return { series, append, clear }
}
