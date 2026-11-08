import type {
  MemberRefundQuote,
  OperationReceipt,
} from './types'
import { request } from './http'

export const memberOperationsApi = {
  getPurchaseRefundQuote: (purchaseId: string) => request<MemberRefundQuote>(`/api/v1/purchases/${encodeURIComponent(purchaseId)}/refund`),
  refundPurchase: (purchaseId: string, idempotencyKey: string) => request<OperationReceipt>(`/api/v1/purchases/${encodeURIComponent(purchaseId)}/refund`, {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
  }),
  getOperation: (operationId: string) => request<OperationReceipt>(`/api/v1/operations/${encodeURIComponent(operationId)}`, { cache: 'no-store', timeoutMilliseconds: 30_000 }),
}
