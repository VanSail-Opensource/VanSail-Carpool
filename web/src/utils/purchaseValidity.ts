import type { Purchase } from '@/api/types'

export type PurchaseValidity = Pick<Purchase, 'status' | 'validFrom' | 'validUntil'>

// Display validity is independent of the durable settlement/activation workflow.
export function effectivePurchaseStatus(purchase: PurchaseValidity, now: number): Purchase['status'] {
  if (!['active', 'activating', 'queued'].includes(purchase.status)) return purchase.status
  if (now >= Date.parse(purchase.validUntil)) return 'expired'
  if (purchase.status !== 'queued' && now < Date.parse(purchase.validFrom)) return 'queued'
  return purchase.status
}

export function isCurrentPurchase(purchase: PurchaseValidity | null | undefined, now: number): boolean {
  return Boolean(purchase && ['active', 'activating'].includes(purchase.status)
    && Date.parse(purchase.validFrom) <= now && now < Date.parse(purchase.validUntil))
}
