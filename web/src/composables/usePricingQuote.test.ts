import { effectScope } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/http'
import { usePricingQuote } from './usePricingQuote'

afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })
it('bounds pending retries, pauses while hidden and cancels on disposal', async () => {
  vi.useFakeTimers()
  const hidden = vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  const request = vi.fn().mockRejectedValue(new ApiError(409, { code: 'PRICING_DECISION_PENDING', message: 'pending' }))
  const scope = effectScope()
  const state = scope.run(() => usePricingQuote<{ quoteVersion: string; quoteValidUntil?: string }, string>({ selection: () => 'basket', fingerprint: () => 'basket', request, onError: () => {} }))!
  await state.refreshQuote()
  expect(state.quoteUsable.value).toBe(false)
  hidden.mockReturnValue(true); document.dispatchEvent(new Event('visibilitychange'))
  await vi.advanceTimersByTimeAsync(30_000)
  expect(request).toHaveBeenCalledTimes(1)
  hidden.mockReturnValue(false); document.dispatchEvent(new Event('visibilitychange'))
  await vi.advanceTimersByTimeAsync(90_001)
  expect(state.quoteStatus.value).toBe('delayed')
  const calls = request.mock.calls.length
  await vi.advanceTimersByTimeAsync(90_000)
  expect(request).toHaveBeenCalledTimes(calls)
  scope.stop()
  expect(vi.getTimerCount()).toBe(0)
})
it('ignores obsolete responses and expires the reviewed total', async () => {
  vi.useFakeTimers()
  let key = 'a'
  let resolve!: (value: { quoteVersion: string; quoteValidUntil?: string }) => void
  const request = vi.fn().mockImplementationOnce(() => new Promise(done => { resolve = done })).mockResolvedValue({ quoteVersion: 'b', quoteValidUntil: new Date(Date.now() + 1000).toISOString() })
  const scope = effectScope()
  const state = scope.run(() => usePricingQuote<{ quoteVersion: string; quoteValidUntil?: string }, string>({ selection: () => key, fingerprint: () => key, request, onError: () => {} }))!
  const obsolete = state.refreshQuote()
  key = 'b'; await state.refreshQuote()
  resolve({ quoteVersion: 'a' }); await obsolete
  expect(state.quote.value?.quoteVersion).toBe('b')
  await vi.advanceTimersByTimeAsync(1001)
  expect(state.quoteStatus.value).toBe('stale')
  expect(state.quoteUsable.value).toBe(false)
  scope.stop()
})
