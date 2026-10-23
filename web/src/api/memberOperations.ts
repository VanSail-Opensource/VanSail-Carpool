import type {
  ConnectionScan,
  IPBlock,
  MemberRefundQuote,
  OperationReceipt,
  TrafficResetAutomation,
} from './types'
import { request } from './http'

export const memberOperationsApi = {
  requestConnections: (idempotencyKey: string) => request<ConnectionScan>('/api/v1/subscription/connections', {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
  }),
  pollConnections: (scanId: string) => request<ConnectionScan>(`/api/v1/subscription/connections/${encodeURIComponent(scanId)}`),
  dropConnection: (handle: string, idempotencyKey: string) => request<OperationReceipt>('/api/v1/subscription/connections/drop', {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: { handle },
  }),
  listIPBlocks: () => request<{ items: IPBlock[] }>('/api/v1/subscription/ip-blocks'),
  unblockIP: (blockId: string, idempotencyKey: string) => request<OperationReceipt>(`/api/v1/subscription/ip-blocks/${encodeURIComponent(blockId)}/unblock`, {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
  }),
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
