import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { featuresApi, type EmbyOverview } from '@/api/features'
import { useEmby } from './useEmby'

const overview = (username: string): EmbyOverview => ({ configured: true, setupPrice: { minor: '100', display: '1.00 TXB', currency: 'TXB' }, ratings: [], libraries: [], account: { id: username, username, status: 'active', maxParentalRating: null, disabledLibraryIds: [], retryable: false, updatedAt: '2026-11-13T00:00:00Z' } })
function setup() {
  let state!: ReturnType<typeof useEmby>
  const wrapper = mount(defineComponent({ setup() { state = useEmby(); return () => h('div') } }))
  return { state, wrapper }
}
beforeEach(() => vi.spyOn(featuresApi, 'getEmby').mockResolvedValue(overview('current')))
afterEach(() => vi.restoreAllMocks())

describe('Emby view request ownership', () => {
  it('keeps a newer account snapshot after the first load finishes late', async () => {
    let finish!: (value: EmbyOverview) => void
    vi.mocked(featuresApi.getEmby).mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const { state, wrapper } = setup()
    await state.load()
    finish(overview('stale'))
    await flushPromises()
    expect(state.overview.value?.account?.username).toBe('current')
    expect(state.loading.value).toBe(false)
    wrapper.unmount()
  })

  it('retires a pending reply and prevents post-disposal loads and writes', async () => {
    let finish!: (value: EmbyOverview) => void
    vi.mocked(featuresApi.getEmby).mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const write = vi.spyOn(featuresApi, 'changeEmbyPassword')
    const { state, wrapper } = setup()
    wrapper.unmount()
    finish(overview('previous-owner'))
    await flushPromises()
    await state.load()
    expect(await state.changePassword('replacement-password')).toBe(false)
    expect(featuresApi.getEmby).toHaveBeenCalledOnce()
    expect(write).not.toHaveBeenCalled()
    expect(state.overview.value).toBeNull()
    expect(state.message.value).toBeNull()
  })
})
