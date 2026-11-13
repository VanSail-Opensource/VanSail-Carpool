import { effectScope } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useClipboard } from './useClipboard'

const { copyText } = vi.hoisted(() => ({ copyText: vi.fn() }))
vi.mock('@/utils/browserCompatibility', () => ({ copyText }))
beforeEach(() => { vi.useFakeTimers(); copyText.mockReset() })
afterEach(() => vi.useRealTimers())

describe('clipboard feedback ownership', () => {
  it('does not let a late earlier success replace the latest failure', async () => {
    let finish!: (value: boolean) => void
    copyText.mockImplementationOnce(() => new Promise<boolean>(resolve => { finish = resolve }))
      .mockResolvedValueOnce(false)
    const scope = effectScope()
    const state = scope.run(() => useClipboard())!
    const older = state.copy('old')
    expect(await state.copy('current')).toBe(false)
    finish(true)
    expect(await older).toBe(false)
    expect(state.copied.value).toBe(false)
    expect(vi.getTimerCount()).toBe(0)
    scope.stop()
  })

  it('invalidates pending feedback on owner reset and releases its timer on disposal', async () => {
    let finish!: (value: boolean) => void
    copyText.mockImplementationOnce(() => new Promise<boolean>(resolve => { finish = resolve }))
      .mockResolvedValueOnce(true)
    const scope = effectScope()
    const state = scope.run(() => useClipboard())!
    const pending = state.copy('old-owner')
    state.reset()
    finish(true)
    expect(await pending).toBe(false)
    expect(state.copied.value).toBe(false)

    expect(await state.copy('new-owner')).toBe(true)
    expect(vi.getTimerCount()).toBe(1)
    scope.stop()
    expect(state.copied.value).toBe(false)
    expect(vi.getTimerCount()).toBe(0)
    expect(await state.copy('disposed')).toBe(false)
    expect(copyText).toHaveBeenCalledTimes(2)
  })
})
