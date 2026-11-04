import type {
  MemberRefundQuote,
  OperationReceipt,
  TrafficResetAutomation,
} from './types'
import { request } from './http'

export const memberOperationsApi = {
  getTrafficResetAutomation: () => request<TrafficResetAutomation>('/api/v1/me/traffic-reset-automation'),
  updateTrafficResetAutomation: (enabled: boolean) => request<TrafficResetAutomation>('/api/v1/me/traffic-reset-automation', {
    method: 'PUT',
    body: { enabled },
  }),
  getPurchaseRefundQuote: (purchaseId: string) => request<MemberRefundQuote>(`/api/v1/purchases/${encodeURIComponent(purchaseId)}/refund`),
  refundPurchase: (purchaseId: string, idempotencyKey: string) => request<OperationReceipt>(`/api/v1/purchases/${encodeURIComponent(purchaseId)}/refund`, {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
  }),
  getOperation: (operationId: string) => request<OperationReceipt>(`/api/v1/operations/${encodeURIComponent(operationId)}`, { cache: 'no-store', timeoutMilliseconds: 30_000 }),
}
