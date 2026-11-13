import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { ActivityResult, LuckyDraw } from '@/api/features'
import { useDrawExperience } from './useDrawExperience'

const preference = vi.hoisted(() => ({ reduced: false }))
vi.mock('@/composables/useMotionPreferences', () => ({
  useMotionPreferences: () => ({ reducedMotion: { get value() { return preference.reduced } } }),
}))
vi.mock('@/i18n', () => ({ localizedError: () => 'Draw failed' }))

const draw: LuckyDraw = {
  id: 'draw-1', name: 'Autumn draw', description: '', feeTxbMinor: '100',
  enabled: true, prizes: [{ id: 'prize-1', name: '50 TXB' }],
}
const result: ActivityResult = {
  id: 'result-1', kind: 'draw', outcome: 'complete', message: '',
  reward: { kind: 'txb_delta', txbDeltaMinor: '5000' },
  drawId: 'draw-1', prizeId: 'prize-1', prizeName: '50 TXB',
  balanceAfter: { currency: 'TXB', minor: '5000', display: '50.00 TXB' },
  createdAt: '2026-09-25T00:00:00Z',
}

function harness(play: (drawId: string) => Promise<ActivityResult>) {
  let experience!: ReturnType<typeof useDrawExperience>
  const wrapper = mount(defineComponent({
    setup() {
      experience = useDrawExperience(play)
      return () => h('div')
    },
  }))
  return { wrapper, experience }
}

afterEach(() => {
  preference.reduced = false
  vi.useRealTimers()
})

describe('draw experience', () => {
  it('does not restore a prize or error after its presentation is disposed', async () => {
    let resolve!: (value: ActivityResult) => void
    const play = vi.fn(() => new Promise<ActivityResult>(done => { resolve = done }))
    const { wrapper, experience } = harness(play)
    experience.begin(draw, 'wheel')
    wrapper.unmount()
    resolve(result)
    await flushPromises()
    expect(experience.draw.value).toBeNull()
    expect(experience.result.value).toBeNull()
    expect(experience.error.value).toBeNull()
    experience.begin(draw, 'simple')
    expect(play).toHaveBeenCalledTimes(1)
  })

  it('waits for the authoritative result and can reveal or reset it', async () => {
    let resolve!: (value: ActivityResult) => void
    const play = vi.fn(() => new Promise<ActivityResult>((done) => { resolve = done }))
    const { wrapper, experience } = harness(play)
    experience.begin(draw, 'wheel')
    expect(experience.phase.value).toBe('running')
    expect(experience.result.value).toBeNull()
    resolve(result)
    await flushPromises()
    expect(experience.phase.value).toBe('settling')
    expect(experience.result.value?.prizeId).toBe('prize-1')
    experience.reveal()
    expect(experience.phase.value).toBe('receipt')
    experience.close()
    expect(experience.draw.value).toBeNull()
    expect(experience.resetToken.value).toBe(1)
    wrapper.unmount()
  })

  it('retries the same draw after a failed request', async () => {
    const play = vi.fn().mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce(result)
    const { wrapper, experience } = harness(play)
    experience.begin(draw, 'scratch')
    await flushPromises()
    expect(experience.phase.value).toBe('error')
    experience.retry()
    await flushPromises()
    expect(play).toHaveBeenCalledTimes(2)
    expect(play).toHaveBeenNthCalledWith(1, 'draw-1')
    expect(play).toHaveBeenNthCalledWith(2, 'draw-1')
    expect(experience.style.value).toBe('scratch')
    expect(experience.phase.value).toBe('settling')
    wrapper.unmount()
  })

  it('keeps the lever gate under reduced motion', async () => {
    preference.reduced = true
    const { wrapper, experience } = harness(async () => result)
    experience.begin(draw, 'slot')
    await flushPromises()
    expect(experience.phase.value).toBe('settling')
    experience.markStarted()
    experience.reveal()
    expect(experience.phase.value).toBe('receipt')
    wrapper.unmount()
  })

  it('reveals a recorded result if an animation never finishes', async () => {
    vi.useFakeTimers()
    const { wrapper, experience } = harness(async () => result)
    experience.begin(draw, 'grid')
    await Promise.resolve()
    await nextTick()
    expect(experience.phase.value).toBe('settling')
    vi.advanceTimersByTime(6000)
    expect(experience.phase.value).toBe('settling')
    experience.markStarted()
    vi.advanceTimersByTime(6000)
    expect(experience.phase.value).toBe('receipt')
    wrapper.unmount()
  })
})
