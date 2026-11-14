import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ActivityOverview, ActivityResult } from '@/api/features'
import { setCacheSession } from '@/api/cache/session'
import { useActivity } from './useActivity'

const mocks = vi.hoisted(() => ({
  getActivity: vi.fn(),
  drawLuckyPrize: vi.fn(),
  checkIn: vi.fn(),
  placeBet: vi.fn(),
  createUuid: vi.fn(),
  notifyHaptic: vi.fn(),
  restoreRef: vi.fn(),
}))
vi.mock('@/api/features', () => ({
  featuresApi: { getActivity: mocks.getActivity, drawLuckyPrize: mocks.drawLuckyPrize, checkIn: mocks.checkIn, placeBet: mocks.placeBet },
}))
vi.mock('@/api/cache/restore', () => ({ restoreRef: mocks.restoreRef }))
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

beforeEach(() => { mocks.restoreRef.mockReset(); mocks.restoreRef.mockReturnValue(false) })
afterEach(() => { vi.clearAllMocks(); setCacheSession(null) })

const overview: ActivityOverview = {
  groupBoost: { state: 'boosted', count: 2, boostUrl: null }, balance: result.balanceAfter,
  timeZone: 'Asia/Shanghai', checkedInToday: false, dailyRewardMinTxbMinor: '100', dailyRewardMaxTxbMinor: '200',
  games: [], draws: [], recentResults: [],
  groupMessageReward: { enabled: true, localDate: '2026-11-14', messageCount: 1, threshold: 10, rewardMinor: '100', rewarded: false },
}

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
  it('renders a cached overview but blocks every mutation until a fresh read succeeds', async () => {
    mocks.restoreRef.mockImplementationOnce((_path, target) => { target.value = overview; return true })
    let resolve!: (value: ActivityOverview) => void
    mocks.getActivity.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const { activity, wrapper } = mountActivity()
    expect(activity.loading.value).toBe(false)
    expect(activity.overview.value).toEqual(overview)
    expect(activity.actionsDisabled.value).toBe(true)
    await activity.checkIn()
    await activity.placeBet({ gameId: 'game-1', stakeTxbMinor: '100' })
    await expect(activity.draw('draw-1')).rejects.toThrow()
    expect(mocks.checkIn).not.toHaveBeenCalled()
    expect(mocks.placeBet).not.toHaveBeenCalled()
    expect(mocks.drawLuckyPrize).not.toHaveBeenCalled()
    resolve(overview)
    await flushPromises()
    expect(activity.actionsDisabled.value).toBe(false)
    wrapper.unmount()
  })

  it('keeps cached actions disabled after a failed refresh', async () => {
    mocks.restoreRef.mockImplementationOnce((_path, target) => { target.value = overview; return true })
    mocks.getActivity.mockRejectedValueOnce(new Error('offline'))
    const { activity, wrapper } = mountActivity()
    await flushPromises()
    expect(activity.loading.value).toBe(false)
    expect(activity.actionsDisabled.value).toBe(true)
    expect(activity.overview.value?.groupBoost.state).toBe('unavailable')
    wrapper.unmount()
  })

  it('ignores an activity reply after the account identity changes', async () => {
    setCacheSession('activity:first')
    let resolve!: (value: ActivityOverview) => void
    mocks.getActivity.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const { activity, wrapper } = mountActivity()
    setCacheSession('activity:second')
    resolve(overview)
    await flushPromises()
    expect(activity.overview.value).toBeNull()
    await activity.checkIn()
    expect(mocks.checkIn).not.toHaveBeenCalled()
    expect(activity.actionsDisabled.value).toBe(true)
    mocks.getActivity.mockResolvedValueOnce(overview)
    await activity.load()
    expect(activity.overview.value).toEqual(overview)
    expect(activity.actionsDisabled.value).toBe(false)
    wrapper.unmount()
  })

  it('synchronously clears a previously enabled view when its account identity changes', async () => {
    setCacheSession('activity:ready')
    mocks.getActivity.mockResolvedValueOnce(overview)
    const { activity, wrapper } = mountActivity()
    await flushPromises()
    expect(activity.actionsDisabled.value).toBe(false)
    expect(activity.overview.value).toEqual(overview)
    setCacheSession('activity:replacement')
    expect(activity.overview.value).toBeNull()
    expect(activity.result.value).toBeNull()
    expect(activity.actionsDisabled.value).toBe(true)
    await activity.checkIn()
    expect(mocks.checkIn).not.toHaveBeenCalled()
    wrapper.unmount()
  })

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
