import { defineComponent, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { IPLookupSharedProgress } from '@/api/ipLookup'

const mocks = vi.hoisted(() => ({ progress: vi.fn() }))
vi.mock('@/api/ipLookup', () => ({ ipLookupApi: mocks }))
const session = reactive({ user: { id: 'member' } })
vi.mock('@/stores/session', () => ({ useSessionStore: () => session }))
import { useSharedIPProgress } from './useSharedIPProgress'

describe('shared IP progress request ownership', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
  })
  afterEach(() => { vi.useRealTimers() })

  it('lets the latest manual refresh finish before scheduling another poll', async () => {
    const running: IPLookupSharedProgress = {
      ip: '8.8.8.8', status: 'processing', updatedAt: '2026-11-13T00:00:00Z',
      progress: { phase: 'topology', completed: 1, total: 8 },
    }
    mocks.progress.mockResolvedValueOnce(running)
    let state!: ReturnType<typeof useSharedIPProgress>
    const wrapper = mount(defineComponent({ setup() {
      state = useSharedIPProgress(() => running.ip, vi.fn())
      return () => null
    } }))
    await flushPromises()
    let complete!: (value: IPLookupSharedProgress | null) => void
    mocks.progress.mockImplementationOnce((_ip, signal: AbortSignal) => new Promise((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')), { once: true })
    })).mockImplementationOnce(() => new Promise(resolve => { complete = resolve }))
    const retired = state.reloadProgress()
    const latest = state.reloadProgress()
    await retired
    const currentSignal = mocks.progress.mock.calls[2]![1] as AbortSignal
    await vi.advanceTimersByTimeAsync(10_000)
    expect(mocks.progress).toHaveBeenCalledTimes(3)
    expect(currentSignal.aborted).toBe(false)
    expect(state.progressUnavailable.value).toBe(false)
    complete(running)
    await latest
    mocks.progress.mockResolvedValueOnce(null)
    await vi.advanceTimersByTimeAsync(5000)
    expect(mocks.progress).toHaveBeenCalledTimes(4)
    expect(state.activeRun.value).toBeNull()
    wrapper.unmount()
    await state.reloadProgress()
    expect(mocks.progress).toHaveBeenCalledTimes(4)
  })
})
