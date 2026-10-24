import { describe, expect, it } from 'vitest'
import { effectivePurchaseStatus, isCurrentPurchase, type PurchaseValidity } from './purchaseValidity'

const now = Date.parse('2026-10-25T12:00:00Z')
const term: PurchaseValidity = { status: 'active', validFrom: '2026-10-24T12:00:00Z', validUntil: '2026-10-25T12:00:00Z' }

describe('purchase display validity', () => {
  it.each(['active', 'activating', 'queued'] as const)('expires %s at the exclusive end boundary', status => {
    const purchase = { ...term, status }
    expect(effectivePurchaseStatus(purchase, now - 1)).toBe(status)
    expect(effectivePurchaseStatus(purchase, now)).toBe('expired')
    expect(isCurrentPurchase(purchase, now)).toBe(false)
    expect(purchase.status).toBe(status)
  })

  it.each(['active', 'activating'] as const)('shows future %s terms as queued', status => {
    const purchase = { ...term, status, validFrom: '2026-10-25T13:00:00Z', validUntil: '2026-10-26T13:00:00Z' }
    expect(effectivePurchaseStatus(purchase, now)).toBe('queued')
    expect(isCurrentPurchase(purchase, now)).toBe(false)
    expect(isCurrentPurchase(purchase, Date.parse(purchase.validFrom))).toBe(true)
  })

  it.each(['cancelled', 'failed', 'expired'] as const)('preserves terminal %s', status => {
    expect(effectivePurchaseStatus({ ...term, status }, now)).toBe(status)
  })

  it('isolates an expired predecessor from its current successor', () => {
    const successor = { ...term, validFrom: term.validUntil, validUntil: '2026-11-25T12:00:00Z' }
    expect(isCurrentPurchase(term, now)).toBe(false)
    expect(isCurrentPurchase(successor, now)).toBe(true)
  })
})
