import { api } from '@/api/client'
import { pricingApi, type PricingRenewalInfo } from '@/api/pricing'
import type { AutoRenewal, Purchase } from '@/api/types'

export function installRenewalPricingFixture(purchase: Purchase, params: URLSearchParams): void {
  const state = { pricingReads: 0, pricingFinished: 0, renewalReads: 0, writes: 0, error: params.has('pricing-error') }
  const money = (minor: string) => ({ currency: 'TXB' as const, minor, display: '' })
  const renewal: AutoRenewal = { purchaseId: purchase.id, enabled: false, canEnable: true, ineligibleReason: null, grossPrice: money('1000'), discount: money('200'), netPrice: money('800'), scheduledAt: purchase.validUntil, nextCycleEndsAt: new Date(Date.now() + 172800_000).toISOString() }
  let release: (() => void) | undefined
  api.getAutoRenewal = async () => { state.renewalReads++; return renewal }
  api.setAutoRenewal = async (_id, enabled) => { state.writes++; return { ...renewal, enabled } }
  pricingApi.renewalInfo = async () => {
    state.pricingReads++
    if (state.error) throw new Error('Constructed renewal price failure')
    const now = Date.now(), date = (offset: number) => new Date(now + offset).toISOString()
    const first = params.has('defer-pricing') && state.pricingReads === 1
    const info: PricingRenewalInfo = {
      policy: { productType: 'combo', productId: purchase.comboId, dynamicEnabled: true, purchasePricingMode: 'DYNAMIC', renewalPricingMode: 'DYNAMIC', fixedPriceMinor: '1000', maxIncreaseBps: 0, maxDiscountBps: 0, overrideStatus: 'NONE', lastApprovedPriceVersion: '', version: 1, updatedBy: 'constructed-admin', updatedAt: date(0) },
      contract: { id: 'constructed-contract', purchaseId: purchase.id, userId: 'constructed-owner', productId: purchase.comboId, mode: 'DYNAMIC', contractualPriceMinor: '1000', priceLimitMinor: '1200', rateLimitBps: 2000, policyVersion: 1, version: 2, sourcePurchaseId: '', recordedAt: date(-1000) },
      quote: { id: `constructed-quote-${state.pricingReads}`, purchaseId: purchase.id, contractId: 'constructed-contract', priceMinor: '1000', policyVersion: 1, contractVersion: 2, state: first ? 'CANCELLED' : params.has('deferred-health') ? 'DEFERRED_HEALTH' : 'AUTHORIZED', generatedAt: date(-1000), authorizedAt: date(-1000), settleAfter: purchase.validUntil, validUntil: date(params.has('expires') ? 3500 : 86400_000), settledPurchaseId: null, probability: 0.8, contribution: 10, healthState: 'HEALTHY', modelVersion: 'constructed-model', trace: '' },
      graceSeconds: 259200,
    }
    if (first) await new Promise<void>(resolve => { release = resolve })
    state.pricingFinished++
    return info
  }
  Object.assign(globalThis, { __renewalPricingAudit: { state, release: () => release?.() } })
}
