import { request } from './http'
import type { components } from './generated'

export interface PricingPolicy {
  productType: 'combo' | 'payg' | 'addon'
  productId: string
  dynamicEnabled: boolean
  purchasePricingMode: 'FIXED' | 'DYNAMIC'
  renewalPricingMode: 'FIXED' | 'DYNAMIC'
  fixedPriceMinor: string
  maxIncreaseBps: number
  maxDiscountBps: number
  overrideStatus: 'NONE' | 'FROZEN' | 'MANUAL'
  lastApprovedPriceVersion: string
  version: number
  updatedBy: string
  updatedAt: string
}

const productPath = (kind: string, id: string) => `/api/v1/admin/pricing/products/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`
export interface PricingRenewalInfo {
  contract: components['schemas']['PricingContract'] | null
  policy: PricingPolicy
  quote: components['schemas']['PricingRenewalQuote'] | null
  graceSeconds: number
}
export const pricingApi = {
  renewalInfo: (id: string, signal?: AbortSignal) => request<PricingRenewalInfo>(`/api/v1/purchases/${encodeURIComponent(id)}/pricing`, { signal }),
  policy: (kind: string, id: string, signal?: AbortSignal) => request<PricingPolicy>(productPath(kind, id), { signal }),
  savePolicy: (policy: PricingPolicy, reason: string) => request<PricingPolicy>(productPath(policy.productType, policy.productId), { method: 'PUT', body: { policy, reason } }),
}
