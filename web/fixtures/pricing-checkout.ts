import { api, ApiError } from '@/api/client'
import { featuresApi } from '@/api/features'
import type { PurchaseQuote } from '@/api/types'
export function installPricingCheckout(params: URLSearchParams) {
 let version = 1
 let ready = !params.has('pending')
 let changed = false
 const receipts=new Set<string>()
 let lost=false
 const audit = { quotes: 0, commits: 0, charges: 0, versions: [] as (string | undefined)[], ready: () => { ready = true } }
 const money = (minor: string) => ({ currency: 'TXB' as const, minor, display: '' })
 api.getCatalog = async () => ({ autoRenewalBlockedUntil: null, combos: [{ id: 'checkout-combo', name: 'Tokyo monthly', description: '', price: money('1000'), validityDays: 30, trafficLimitBytes: '107374182400', resetStrategy: 'MONTH_ROLLING', includedSquads: [], active: true, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }], addons: [], nodes: [] })
 api.getBalance = async () => ({ balance: money('5000'), paymentMethods: [], addAmountLimits: {}, pendingPaymentOrder: null }) as never
 featuresApi.getCouponWallet = async () => ({ items: [] }) as never
 api.quotePurchase = async () => {
  audit.quotes++
  if (!ready) throw new ApiError(409, { code: 'PRICING_DECISION_PENDING', message: '' })
  if (params.has('unavailable')) throw new Error('Constructed preparation failure')
  const quote: PurchaseQuote = { quoteVersion: String(version), quoteValidUntil: new Date(Date.now() + (params.has('expire') ? 4000 : 300000)).toISOString(), comboId: 'checkout-combo', comboName: 'Tokyo monthly', grossPrice: money(version === 1 ? '1000' : '1100'), discount: money('0'), netPrice: money(version === 1 ? '1000' : '1100'), effectiveAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 30 * 86400000).toISOString(), queued: false, addonSquadUuids: [], accessibleNodes: [{ uuid: 'node', name: 'Tokyo', countryCode: 'JP', consumptionMultiplier: 1, activeInboundUuids: [], accessible: true, providerName: null, providerFaviconUrl: null }] }
  return quote
 }
 api.createPurchase = async (_combo, _squads, _coupon, _key, _codes, reviewed) => {
  audit.commits++; audit.versions.push(reviewed)
  if (_key && receipts.has(_key)) return { id: 'checkout-purchase', status: 'active' } as never
  if (params.has('changed') && !changed) { changed = true; version++; throw new ApiError(409, { code: 'PRICING_QUOTE_CHANGED', message: '' }) }
  if (reviewed !== String(version)) throw new ApiError(409, { code: 'PRICING_QUOTE_CHANGED', message: '' })
  audit.charges++; if (_key) receipts.add(_key)
  if (params.has('lost') && !lost) { lost=true; throw new Error('Constructed lost reply after commit') }
  return { id: 'checkout-purchase', status: 'active' } as never
 }
 Object.assign(window, { pricingCheckoutAudit: audit })
}
