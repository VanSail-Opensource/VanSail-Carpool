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
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  let playback!: ReturnType<typeof useIllustrationPlayback>
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
  } finally { wrapper.unmount() }
  expect(vi.getTimerCount()).toBe(0)
})
