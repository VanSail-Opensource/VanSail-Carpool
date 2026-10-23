import { afterEach, describe, expect, it, vi } from 'vitest'
import { withRequestDeadline } from './requestDeadline'

afterEach(() => vi.useRealTimers())

describe('finite API deadlines', () => {
  it('aborts a stalled request and ignores a late result', async () => {
    vi.useFakeTimers()
    let requestSignal: AbortSignal | undefined
    let finish: ((value: string) => void) | undefined
    const pending = withRequestDeadline(30_000, undefined, signal => {
      requestSignal = signal
      return new Promise<string>(resolve => { finish = resolve })
    })
    const rejected = expect(pending).rejects.toMatchObject({ name: 'TimeoutError' })
    await vi.advanceTimersByTimeAsync(30_000)
    await rejected
    expect(requestSignal?.aborted).toBe(true)
    finish?.('late response')
    expect(vi.getTimerCount()).toBe(0)
  })

  it('preserves parent cancellation and never starts an already aborted request', async () => {
    const parent = new AbortController()
    parent.abort()
    const load = vi.fn()
    await expect(withRequestDeadline(30_000, parent.signal, load)).rejects.toMatchObject({ name: 'AbortError' })
    expect(load).not.toHaveBeenCalled()
  })

  it('cleans up after a successful request', async () => {
    vi.useFakeTimers()
    await expect(withRequestDeadline(30_000, undefined, async () => 'ready')).resolves.toBe('ready')
    expect(vi.getTimerCount()).toBe(0)
  })
})
