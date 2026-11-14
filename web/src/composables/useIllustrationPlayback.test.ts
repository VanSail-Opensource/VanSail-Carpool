import { defineComponent, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import { useIllustrationPlayback } from './useIllustrationPlayback'

vi.mock('./useMotionPreferences', async () => {
  const { shallowRef } = await import('vue')
  return { useMotionPreferences: () => ({ reducedMotion: shallowRef(false) }) }
})
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

it('restarts the full first-step interval when Replay is pressed during step zero', async () => {
  vi.useFakeTimers()
  const schedule = vi.spyOn(globalThis, 'setTimeout')
  const clear = vi.spyOn(globalThis, 'clearTimeout')
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  let playback!: ReturnType<typeof useIllustrationPlayback>
  let ownedTimer: ReturnType<typeof globalThis.setTimeout> | undefined
  const wrapper = mount(defineComponent({ setup() {
    playback = useIllustrationPlayback({ active: true, stepCount: 3, interval: 1000 })
    return () => null
  } }))
  try {
    await vi.advanceTimersByTimeAsync(750)
    expect(playback.step.value).toBe(0)
    playback.replay()
    await nextTick()
    await vi.advanceTimersByTimeAsync(500)
    expect(playback.step.value).toBe(0)
    await vi.advanceTimersByTimeAsync(500)
    expect(playback.step.value).toBe(1)
    const ownedIndex = schedule.mock.calls.map(([, delay]) => delay).lastIndexOf(1000)
    ownedTimer = schedule.mock.results[ownedIndex]?.value
    expect(ownedTimer).toBeDefined()
    clear.mockClear()
  } finally { wrapper.unmount() }
  // Vue's development-only 3,000 ms devtools fallback also runs in Happy DOM.
  // Verify our pending handle is cleared without counting framework timers.
  expect(clear).toHaveBeenCalledWith(ownedTimer)
  await vi.advanceTimersByTimeAsync(3000)
  expect(playback.step.value).toBe(1)
})
