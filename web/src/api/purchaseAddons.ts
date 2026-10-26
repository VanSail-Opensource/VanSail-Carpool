import type { components } from './generated'
import type { Purchase, PurchaseAddonQuote } from './types'
import { request } from './http'

type PurchaseAddonRequest = components['schemas']['PurchaseAddonRequest']

export const purchaseAddonsApi = {
  quotePurchaseAddons: (purchaseId: string, squadProductIds: string[], couponGrantId?: string) => request<PurchaseAddonQuote>(
    `/api/v1/purchases/${encodeURIComponent(purchaseId)}/addons/quote`,
    { method: 'POST', body: { addonSquadProductIds: squadProductIds, ...(couponGrantId ? { couponGrantId } : {}) } satisfies PurchaseAddonRequest },
  ),
  addPurchaseAddons: (purchaseId: string, squadProductIds: string[], idempotencyKey: string, squadActivationCodes: Record<string, string> = {}, couponGrantId?: string) => request<Purchase>(
    `/api/v1/purchases/${encodeURIComponent(purchaseId)}/addons`,
    {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: {
        addonSquadProductIds: squadProductIds,
        ...(couponGrantId ? { couponGrantId } : {}),
        ...(Object.keys(squadActivationCodes).length > 0 ? { squadActivationCodes } : {}),
      } satisfies PurchaseAddonRequest,
    },
  ),
}
