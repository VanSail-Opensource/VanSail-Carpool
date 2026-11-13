import { describe, expect, it } from 'vitest'
import type { PricingRenewalInfo } from '@/api/pricing'
import type { AutoRenewal } from '@/api/types'
import { renewalPriceState } from './renewalPricing'

const now = Date.parse('2026-11-13T00:00:00Z')
const date = (offset: number) => new Date(now + offset).toISOString()
const money = (minor: string) => ({ currency: 'TXB' as const, minor, display: '' })
const renewal: AutoRenewal = { purchaseId: 'purchase-1', enabled: true, canEnable: true, ineligibleReason: null, grossPrice: money('1000'), discount: money('200'), netPrice: money('800'), scheduledAt: date(3600_000), nextCycleEndsAt: date(86400_000) }
function info(): PricingRenewalInfo {
  return {
    policy: { productType: 'combo', productId: 'combo-1', dynamicEnabled: true, purchasePricingMode: 'DYNAMIC', renewalPricingMode: 'DYNAMIC', fixedPriceMinor: '1000', maxIncreaseBps: 0, maxDiscountBps: 0, overrideStatus: 'NONE', lastApprovedPriceVersion: '', version: 1, updatedBy: 'admin', updatedAt: date(0) },
    contract: { id: 'contract-1', purchaseId: 'purchase-1', userId: 'owner-1', productId: 'combo-1', mode: 'DYNAMIC', contractualPriceMinor: '1000', priceLimitMinor: '1200', rateLimitBps: 2000, policyVersion: 1, version: 2, sourcePurchaseId: '', recordedAt: date(-1000) },
    quote: { id: 'quote-1', purchaseId: 'purchase-1', contractId: 'contract-1', priceMinor: '1000', policyVersion: 1, contractVersion: 2, state: 'AUTHORIZED', generatedAt: date(-1000), authorizedAt: date(-1000), settleAfter: date(3600_000), validUntil: date(86400_000), settledPurchaseId: null, probability: 0.8, contribution: 10, healthState: 'HEALTHY', modelVersion: 'model-1', trace: '' },
    graceSeconds: 259200,
  }
}

describe('authoritative renewal price projection', () => {
  it('shows an authorized future price preview while retaining the server discounted net', () => {
    const value = info()
    expect(renewalPriceState(value, renewal, 'purchase-1', now)).toBe('prepared')
    expect(renewal.netPrice.minor).toBe('800')
    expect(value.quote?.priceMinor).toBe('1000')
  })
  it.each(['CANCELLED', 'EXPIRED', 'SETTLED', 'NOTICE_PENDING', 'unknown'])('never presents %s as a prepared quote', state => {
    const value = info(); value.quote!.state = state
    expect(renewalPriceState(value, renewal, 'purchase-1', now)).toBe('unavailable')
  })
  it('labels health deferral and expires the preview at the exact boundary', () => {
    const value = info(); value.quote!.state = 'DEFERRED_HEALTH'
    expect(renewalPriceState(value, renewal, 'purchase-1', now)).toBe('deferred')
    expect(renewalPriceState(value, renewal, 'purchase-1', Date.parse(value.quote!.validUntil))).toBe('stale')
  })
  it('rejects stale authorization, contract or gross-price evidence across paired responses', () => {
    for (const alter of [
      (value: PricingRenewalInfo) => { value.quote!.authorizedAt = null },
      (value: PricingRenewalInfo) => { value.quote!.contractVersion++ },
      (value: PricingRenewalInfo) => { value.quote!.contractId = 'old-contract' },
      (value: PricingRenewalInfo) => { value.quote!.priceMinor = '1100' },
      (value: PricingRenewalInfo) => { value.quote!.purchaseId = 'another-purchase' },
      (value: PricingRenewalInfo) => { value.quote!.settledPurchaseId = 'already-settled' },
    ]) {
      const value = info(); alter(value)
      expect(renewalPriceState(value, renewal, 'purchase-1', now)).toBe('unavailable')
    }
  })
  it('uses the retained contract even when the current public policy differs', () => {
    const value = info(); value.policy.dynamicEnabled = false; value.policy.renewalPricingMode = 'FIXED'
    expect(renewalPriceState(value, renewal, 'purchase-1', now)).toBe('prepared')
    value.quote = null
    expect(renewalPriceState(value, renewal, 'purchase-1', now)).toBe('calculating')
    value.policy.dynamicEnabled = true; value.policy.renewalPricingMode = 'DYNAMIC'; value.contract!.mode = 'FIXED'
    expect(renewalPriceState(value, renewal, 'purchase-1', now)).toBe('fixed')
    value.contract = null
    expect(renewalPriceState(value, renewal, 'purchase-1', now)).toBe('fixed')
  })
  it('does not establish pricing readiness when either read is unavailable', () => {
    expect(renewalPriceState(null, renewal, 'purchase-1', now)).toBe('unavailable')
    expect(renewalPriceState(info(), null, 'purchase-1', now)).toBe('unavailable')
  })
})
