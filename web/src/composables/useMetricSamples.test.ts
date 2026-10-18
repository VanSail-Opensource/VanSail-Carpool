import { effectScope } from 'vue'
import { describe, expect, it } from 'vitest'
import { useMetricSamples } from './useMetricSamples'

describe('mounted metric series ownership', () => {
  it('keeps only 300 ordered samples for displayed series', () => {
    const scope = effectScope()
    const state = scope.run(useMetricSamples)!
    for (let at = 0; at < 350; at++) state.append(at, { upload: at, download: at * 2 })
    expect(state.series.value.upload).toHaveLength(300)
    expect(state.series.value.upload?.[0]?.at).toBe(50)
    state.append(300, { upload: 999 })
    expect(state.series.value.upload?.at(-1)?.value).toBe(349)
    expect(state.series.value.download).toBeUndefined()
    scope.stop(); expect(state.series.value).toEqual({})
  })
  it('clears logout buffers and refuses late samples from the retired session', () => {
    const scope = effectScope()
    const state = scope.run(useMetricSamples)!
    state.append(1, { upload: 50 })
    window.dispatchEvent(new Event('txc:session-cleared'))
    state.append(2, { upload: 99 })
    expect(state.series.value).toEqual({})
    scope.stop()
  })
})
