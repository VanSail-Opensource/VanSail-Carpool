import { request } from './http'
import type { ComboControls, EarlyActivationQuote, OperationReceipt } from './types'

export const comboControlsApi = {
  get: () => request<ComboControls>('/api/v1/me/internal-squads', { cache: 'no-store' }),
  switchSquad: (purchaseId: string, uuid: string, enabled: boolean, key: string) => request<OperationReceipt>(
    `/api/v1/me/internal-squads/${encodeURIComponent(uuid)}`,
    { method: 'PUT', headers: { 'Idempotency-Key': key }, body: { purchaseId, enabled } },
  ),
  quoteActivation: (queuedId: string) => request<EarlyActivationQuote>(`/api/v1/purchases/${encodeURIComponent(queuedId)}/early-activation`),
  activate: (queuedId: string, currentPurchaseId: string, confirmation: string, key: string) => request<OperationReceipt>(
    `/api/v1/purchases/${encodeURIComponent(queuedId)}/activate`,
    { method: 'POST', headers: { 'Idempotency-Key': key }, body: { currentPurchaseId, confirmation } },
  ),
}
