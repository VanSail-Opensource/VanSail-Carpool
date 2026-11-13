import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { ActivityResult } from '@/api/features'
import { useActivity } from './useActivity'

const mocks = vi.hoisted(() => ({
  getActivity: vi.fn(),
  drawLuckyPrize: vi.fn(),
  checkIn: vi.fn(),
  placeBet: vi.fn(),
  createUuid: vi.fn(),
  notifyHaptic: vi.fn(),
}))
vi.mock('@/api/features', () => ({
  featuresApi: { getActivity: mocks.getActivity, drawLuckyPrize: mocks.drawLuckyPrize, checkIn: mocks.checkIn, placeBet: mocks.placeBet },
}))
vi.mock('@/api/cache/restore', () => ({ restoreRef: () => false }))
vi.mock('@/i18n', () => ({ localizedError: () => 'Draw failed' }))
vi.mock('@/utils/browserCompatibility', () => ({ createUuid: mocks.createUuid }))
vi.mock('@/utils/telegram', () => ({
  notifyHaptic: mocks.notifyHaptic, notifyBetOutcome: vi.fn(),
}))

const result: ActivityResult = {
  id: 'result-1', kind: 'draw', outcome: 'complete', message: '',
  reward: { kind: 'none' }, drawId: 'draw-1', prizeId: 'prize-1', prizeName: 'No prize',
  balanceAfter: { currency: 'TXB', minor: '0', display: '0.00 TXB' },
  createdAt: '2026-09-25T00:00:00Z',
}

afterEach(() => vi.clearAllMocks())

function mountActivity() {
  let activity!: ReturnType<typeof useActivity>
  const wrapper = mount(defineComponent({
    setup() {
      activity = useActivity()
      return () => h('div')
    },
  }))
  return { activity, wrapper }
}

describe('lucky draw submission', () => {
  it.each(['check-in', 'bet', 'draw'] as const)('retires %s replies and refreshes when its view unmounts', async kind => {
    mocks.getActivity.mockResolvedValue({ draws: [], recentResults: [] })
    mocks.createUuid.mockReturnValue('retired-key')
    let resolve!: (value: ActivityResult) => void
    const response = new Promise<ActivityResult>(done => { resolve = done })
    const action = kind === 'draw' ? mocks.drawLuckyPrize : kind === 'bet' ? mocks.placeBet : mocks.checkIn
    action.mockReturnValueOnce(response)
    const { activity, wrapper } = mountActivity()
    await flushPromises()
    const pending = (kind === 'draw' ? activity.draw('draw-1') : kind === 'bet' ? activity.placeBet({ gameId: 'game-1', stakeTxbMinor: '100' }) : activity.checkIn()).catch(error => error)
    wrapper.unmount()
    resolve(result)
    await pending
    await activity.load()
    expect(mocks.getActivity).toHaveBeenCalledTimes(1)
    expect(mocks.notifyHaptic).not.toHaveBeenCalled()
    expect(activity.overview.value).toBeNull()
    expect(activity.result.value).toBeNull()
  })

  it('keeps the same idempotency key after an uncertain failure and clears it after confirmation', async () => {
    mocks.getActivity.mockResolvedValue({ draws: [], recentResults: [] })
    mocks.createUuid.mockReturnValueOnce('key-1').mockReturnValueOnce('key-2')
    mocks.drawLuckyPrize.mockRejectedValueOnce(new Error('network')).mockResolvedValue(result)
    const { activity, wrapper } = mountActivity()
    await flushPromises()

    await expect(activity.draw('draw-1')).rejects.toThrow('network')
    expect(await activity.draw('draw-1')).toEqual(result)
    expect(await activity.draw('draw-1')).toEqual(result)
    expect(mocks.drawLuckyPrize.mock.calls).toEqual([
      ['draw-1', 'key-1'], ['draw-1', 'key-1'], ['draw-1', 'key-2'],
    ])
    expect(activity.result.value).toBeNull()
    wrapper.unmount()
  })
})
