import { defineComponent, nextTick, shallowRef } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import { useIllustrationPlayback } from './useIllustrationPlayback'

const motion = vi.hoisted(() => ({ reduced: undefined as unknown as import('vue').ShallowRef<boolean> }))
vi.mock('./useMotionPreferences', async () => {
  const { shallowRef } = await import('vue')
  motion.reduced = shallowRef(false)
  return { useMotionPreferences: () => ({ reducedMotion: motion.reduced }) }
})
let wrapper: VueWrapper | undefined
afterEach(() => { wrapper?.unmount(); wrapper = undefined; motion.reduced.value = false; vi.restoreAllMocks(); vi.unstubAllGlobals() })

function setup() {
  let now = 0, sequence = 0, hidden = false
  const frames = new Map<number, globalThis.FrameRequestCallback>()
  const active = shallowRef(true)
  const interval = shallowRef(1000)
  vi.spyOn(performance, 'now').mockImplementation(() => now)
  vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
  vi.stubGlobal('requestAnimationFrame', vi.fn((callback: globalThis.FrameRequestCallback) => { frames.set(++sequence, callback); return sequence }))
  vi.stubGlobal('cancelAnimationFrame', vi.fn((handle: number) => frames.delete(handle)))
  let playback!: ReturnType<typeof useIllustrationPlayback>
  wrapper = mount(defineComponent({ setup() {
    playback = useIllustrationPlayback({ active, stepCount: 3, interval })
    return () => null
  } }))
  return {
    playback, frames, active, interval,
    async advance(milliseconds: number) {
      now += milliseconds
      const pending = [...frames.values()]
      frames.clear()
      pending.forEach(callback => callback(now))
      await nextTick()
    },
    async visibility(value: boolean) {
      hidden = value
      document.dispatchEvent(new globalThis.Event('visibilitychange'))
      await nextTick()
    },
  }
}

it('advances within a chapter and resumes from the exact paused frame', async () => {
  const { playback, advance, frames } = setup()
  await advance(250)
  expect(playback.step.value).toBe(0)
  expect(playback.phase.value).toBeCloseTo(.25)
  playback.pause()
  expect(frames.size).toBe(0)
  await advance(5000)
  expect(playback.elapsed.value).toBe(250)
  playback.play()
  await advance(250)
  expect(playback.elapsed.value).toBe(500)
})

it('restarts from zero on replay and disposes the owned animation frame', async () => {
  const { playback, advance, frames } = setup()
  await advance(750)
  playback.replay()
  await advance(500)
  expect(playback.step.value).toBe(0)
  expect(playback.phase.value).toBeCloseTo(.5)
  await advance(500)
  expect(playback.step.value).toBe(1)
  expect(frames.size).toBe(1)
  wrapper?.unmount(); wrapper = undefined
  expect(frames.size).toBe(0)
  await advance(5000)
  expect(playback.elapsed.value).toBe(1000)
})

it('seeks without playing, bounds invalid input and stops at the final frame', async () => {
  const { playback, advance, frames } = setup()
  playback.seek(.5)
  expect(playback.elapsed.value).toBe(1500)
  expect(playback.playing.value).toBe(false)
  playback.seek(Number.NaN)
  expect(playback.elapsed.value).toBe(1500)
  playback.selectStep(999)
  expect(playback.step.value).toBe(2)
  expect(playback.phase.value).toBeCloseTo(.92)
  playback.play()
  await advance(500)
  expect(playback.progress.value).toBe(1)
  expect(playback.phase.value).toBe(1)
  expect(playback.playing.value).toBe(false)
  expect(frames.size).toBe(0)
  playback.play()
  expect(playback.elapsed.value).toBe(0)
})

it('does not skip the story while hidden or continue after the modal closes', async () => {
  const { playback, advance, visibility, frames, active } = setup()
  await advance(300)
  await visibility(true)
  expect(frames.size).toBe(0)
  await advance(10000)
  await visibility(false)
  await advance(200)
  expect(playback.elapsed.value).toBe(500)
  active.value = false
  await nextTick()
  expect(frames.size).toBe(0)
  expect(playback.playing.value).toBe(false)
  active.value = true
  await nextTick()
  expect(playback.elapsed.value).toBe(0)
  expect(frames.size).toBe(1)
})

it('provides completed manual chapter states without scheduling reduced motion', () => {
  motion.reduced.value = true
  const { playback, frames } = setup()
  expect(playback.phase.value).toBeCloseTo(.92)
  expect(playback.playing.value).toBe(false)
  expect(frames.size).toBe(0)
  playback.selectStep(2)
  expect(playback.step.value).toBe(2)
  playback.play()
  expect(frames.size).toBe(0)
  playback.replay()
  expect(playback.step.value).toBe(0)
  expect(playback.phase.value).toBeCloseTo(.92)
})

it('resets to the new lesson interval without retaining the previous runtime', async () => {
  const { playback, interval, advance, frames } = setup()
  await advance(450)
  interval.value = 3200
  await nextTick()
  expect(playback.duration.value).toBe(9600)
  expect(playback.elapsed.value).toBe(0)
  expect(frames.size).toBe(1)
  playback.selectStep(1)
  expect(playback.elapsed.value).toBe(6144)
  motion.reduced.value = true
  interval.value = 2800
  await nextTick()
  expect(playback.duration.value).toBe(8400)
  expect(playback.phase.value).toBeCloseTo(.92)
  expect(frames.size).toBe(0)
})
