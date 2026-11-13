import type { PricingRenewalInfo } from '@/api/pricing'
import type { AutoRenewal } from '@/api/types'

export type RenewalPriceState = 'fixed' | 'calculating' | 'prepared' | 'deferred' | 'stale' | 'unavailable'

// Quotes retain gross prices. The paired renewal read owns coupon-adjusted net
// amounts; presentation must never substitute gross for the member's debit.
export function renewalPriceState(info: PricingRenewalInfo | null, renewal: AutoRenewal | null, purchaseId: string, now: number): RenewalPriceState {
  if (!info || !renewal || renewal.purchaseId !== purchaseId) return 'unavailable'
  const contract = info.contract
  // No retained contract is the legacy basket path, independent of new policies.
  if (!contract) return 'fixed'
  if (contract.purchaseId !== purchaseId) return 'unavailable'
  if (contract.mode === 'FIXED') return 'fixed'
  if (contract.mode !== 'DYNAMIC') return 'unavailable'
  const quote = info.quote
  if (!quote) return 'calculating'
  if (quote.purchaseId !== purchaseId || quote.contractId !== contract.id
    || quote.contractVersion !== contract.version || quote.settledPurchaseId) return 'unavailable'
  const expiry = Date.parse(quote.validUntil)
  if (!Number.isFinite(now) || !Number.isFinite(expiry) || expiry <= now) return 'stale'
  if (!quote.authorizedAt || !Number.isFinite(Date.parse(quote.authorizedAt))) return 'unavailable'
  if (!['AUTHORIZED', 'READY_TO_SETTLE', 'DEFERRED_HEALTH'].includes(quote.state)) return 'unavailable'
  try {
    if (renewal.grossPrice.currency !== 'TXB' || BigInt(quote.priceMinor) !== BigInt(renewal.grossPrice.minor)) return 'unavailable'
  } catch { return 'unavailable' }
  // SettleAfter may be a future due date. Prepared describes the price preview,
  // not settlement admission, which remains subject to current server checks.
  return quote.state === 'DEFERRED_HEALTH' ? 'deferred' : 'prepared'
}
