import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/http'
import type { XNodeSubscription } from '@/api/xnodeMember'
import { usePublicSubscription } from './usePublicSubscription'

const { readMetadata, copyText } = vi.hoisted(() => ({ readMetadata: vi.fn(), copyText: vi.fn() }))
vi.mock('@/api/xnodeMember', () => ({ publicSubscriptionMetadata: readMetadata }))
vi.mock('@/utils/browserClipboard', () => ({ copyText }))

const token = 'a'.repeat(43)
const cleanup: Array<() => void> = []
function metadata(overrides: Partial<XNodeSubscription> = {}): XNodeSubscription {
  return {
    username: 'member', url: '/subscription/' + token,
    downloadUrl: '/api/v1/subscriptions/' + token, usedBytes: '0', limitBytes: '0',
    expiresAt: null, active: true, incomplete: false, ready: true,
    firstSyncedAt: '2026-11-12T00:00:00Z', formats: { base64: true, 'sing-box': true },
    ...overrides,
  }
}
function harness(value = token) {
  let state!: ReturnType<typeof usePublicSubscription>
  const wrapper = mount(defineComponent({
    setup() { state = usePublicSubscription(value); return () => null },
  }))
  let mounted = true
  const unmount = () => { if (mounted) { mounted = false; wrapper.unmount() } }
  cleanup.push(unmount)
  return { state, unmount }
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
  readMetadata.mockReset()
  copyText.mockReset().mockResolvedValue(true)
})
afterEach(() => {
  for (const dispose of cleanup.splice(0)) dispose()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('public bearer subscription recovery', () => {
  it('rejects an invalid bearer without starting a read or timer', async () => {
    const schedule = vi.spyOn(globalThis, 'setTimeout')
    const { state } = harness('invalid')
    await flushPromises()
    expect(readMetadata).not.toHaveBeenCalled()
    expect(state.failed.value).toBe(true)
    expect(state.link.value).toBe('')
    expect(schedule).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(readMetadata).not.toHaveBeenCalled()
  })

  it.each([401, 403, 404, 410])('clears pending metadata and stops polling after HTTP %s', async (status) => {
    readMetadata.mockResolvedValueOnce(metadata({ ready: false, firstSyncedAt: null }))
      .mockRejectedValueOnce(new ApiError(status, { code: 'SUBSCRIPTION_UNAVAILABLE', message: '' }))
    const { state } = harness()
    await flushPromises()
    expect(state.metadata.value?.ready).toBe(false)

    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()
    expect(state.metadata.value).toBeNull()
    expect(state.failed.value).toBe(true)
    expect(state.link.value).toBe('')
    await vi.advanceTimersByTimeAsync(10_000)
    expect(readMetadata).toHaveBeenCalledTimes(2)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('keeps last-good details after 503 but withholds links until explicit retry succeeds', async () => {
    readMetadata.mockResolvedValueOnce(metadata())
      .mockRejectedValueOnce(new ApiError(503, { code: 'SUBSCRIPTION_UNAVAILABLE', message: '' }))
      .mockResolvedValueOnce(metadata({ username: 'current' }))
    const { state } = harness()
    await flushPromises()
    expect(state.link.value).toContain('format=base64')

    await state.load()
    expect(state.metadata.value?.username).toBe('member')
    expect(state.failed.value).toBe(true)
    expect(state.link.value).toBe('')
    expect(state.importLink.value).toBe('')
    await state.copy()
    expect(copyText).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(readMetadata).toHaveBeenCalledTimes(2)

    await state.load()
    expect(state.failed.value).toBe(false)
    expect(state.metadata.value?.username).toBe('current')
    expect(state.link.value).toContain('format=base64')
  })

  it('stops retrying a pending deployment after a transient read failure', async () => {
    readMetadata.mockResolvedValueOnce(metadata({ ready: false, firstSyncedAt: null }))
      .mockRejectedValueOnce(new ApiError(503, { code: 'SUBSCRIPTION_UNAVAILABLE', message: '' }))
    const { state } = harness()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()

    expect(state.metadata.value?.ready).toBe(false)
    expect(state.failed.value).toBe(true)
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(10_000)
    expect(readMetadata).toHaveBeenCalledTimes(2)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('withholds links while refreshing and ignores superseded or disposed results', async () => {
    let finish!: (value: XNodeSubscription) => void
    readMetadata.mockResolvedValueOnce(metadata())
      .mockImplementationOnce(() => new Promise<XNodeSubscription>(resolve => { finish = resolve }))
      .mockResolvedValueOnce(metadata({ username: 'latest' }))
    const { state, unmount } = harness()
    await flushPromises()
    const pending = state.load()
    const oldSignal = readMetadata.mock.calls[1]![1] as AbortSignal
    expect(state.loading.value).toBe(true)
    expect(state.link.value).toBe('')

    await state.load()
    expect(oldSignal.aborted).toBe(true)
    finish(metadata({ username: 'obsolete' }))
    await pending
    expect(state.metadata.value?.username).toBe('latest')
    let finishDisposed!: (value: XNodeSubscription) => void
    readMetadata.mockImplementationOnce(() => new Promise<XNodeSubscription>(resolve => { finishDisposed = resolve }))
    const disposedRead = state.load()
    const disposedSignal = readMetadata.mock.calls.at(-1)![1] as AbortSignal
    unmount()
    expect(disposedSignal.aborted).toBe(true)
    finishDisposed(metadata({ username: 'disposed' }))
    await disposedRead
    expect(state.metadata.value?.username).toBe('latest')
    const calls = readMetadata.mock.calls.length
    await state.load()
    document.dispatchEvent(new Event('visibilitychange'))
    expect(readMetadata).toHaveBeenCalledTimes(calls)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('pauses pending polls while hidden and resumes once on visibility', async () => {
    readMetadata.mockResolvedValueOnce(metadata({ ready: false, firstSyncedAt: null }))
      .mockResolvedValueOnce(metadata())
    const { state } = harness()
    await flushPromises()
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(10_000)
    expect(readMetadata).toHaveBeenCalledTimes(1)

    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(readMetadata).toHaveBeenCalledTimes(2)
    expect(state.link.value).toContain('format=base64')
    expect(vi.getTimerCount()).toBe(0)
  })

  it('discards copy feedback when the selected link changes and later changes back', async () => {
    readMetadata.mockResolvedValue(metadata())
    let finish!: (value: boolean) => void
    copyText.mockImplementationOnce(() => new Promise<boolean>(resolve => { finish = resolve }))
    const { state } = harness()
    await flushPromises()
    const pending = state.copy()
    state.format.value = 'sing-box'
    state.format.value = 'base64'
    finish(true)
    await pending

    expect(state.copied.value).toBe(false)
    expect(state.copyFailed.value).toBe(false)
    await state.copy()
    expect(state.copied.value).toBe(true)
    state.format.value = 'xray'
    expect(state.link.value).toBe('')
    expect(state.copied.value).toBe(false)
  })
})
