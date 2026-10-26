import { afterEach, describe, expect, it, vi } from 'vitest'
import { useCatalogQuote } from './catalogQuoteState'
import { ApiError } from '@/api/http'
const mocks = vi.hoisted(() => ({ quote: vi.fn() }))
vi.mock('@/api/client', async () => { const { ApiError } = await import('@/api/http'); return { api: { quotePurchase: mocks.quote }, ApiError } })
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks() })
describe('pending price decisions', () => {
 it('keeps checkout locked and discards retries after selection changes', async () => {
  vi.useFakeTimers()
  let fingerprint = 'combo-a'
  mocks.quote.mockRejectedValue(new ApiError(409, { code: 'PRICING_DECISION_PENDING', message: 'pending' }))
  const state = useCatalogQuote({ selection: () => ({ comboId: fingerprint, squadProductIds: [] }), fingerprint: () => fingerprint, onError: vi.fn() })
  expect(await state.refreshQuote()).toBe(false)
  expect(state.quoteUsable.value).toBe(false)
  fingerprint = 'combo-b'
  await vi.advanceTimersByTimeAsync(5001)
  expect(mocks.quote).toHaveBeenCalledOnce()
 })
})
