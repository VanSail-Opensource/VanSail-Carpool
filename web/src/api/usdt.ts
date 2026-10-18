import type { components } from './generated'
import { request } from './http'

export type USDTConfig = components['schemas']['USDTConfig']
export type USDTEndpoint = components['schemas']['USDTEndpoint']
export type USDTNetworkConfig = components['schemas']['USDTNetworkConfig']
export type USDTNetworkID = components['schemas']['USDTNetworkID']
export type USDTStatus = components['schemas']['USDTStatus']
export type USDTTransfer = components['schemas']['USDTTransfer']
export type USDTReviewPage = components['schemas']['USDTReviewPage']
export type USDTResolution = components['schemas']['USDTResolution']
export type USDTNetworkStatus = components['schemas']['USDTNetworkStatus']

export const usdtApi = {
  config: (signal?: AbortSignal) => request<USDTConfig>('/api/v1/admin/usdt', { cache: 'no-store', signal }),
  save: (body: USDTConfig) => request<USDTConfig>('/api/v1/admin/usdt', { method: 'PUT', body }),
  status: (signal?: AbortSignal) => request<USDTStatus>('/api/v1/admin/usdt/status', { cache: 'no-store', signal }),
  check: (network: USDTNetworkID, rail?: string) => request<components['schemas']['USDTCheckResult']>('/api/v1/admin/usdt/check', { method: 'POST', body: { network, rail } }),
  review: (cursor?: string, signal?: AbortSignal, orderId?: string) => request<USDTReviewPage>('/api/v1/admin/usdt/review', { cache: 'no-store', signal, query: { cursor, limit: 50, orderId } }),
  dismiss: (id: string, reason: string) => request<USDTResolution>(`/api/v1/admin/usdt/review/${encodeURIComponent(id)}/resolution`, { method: 'PUT', body: { action: 'dismiss', reason } }),
  resolve: (id: string, orderId: string, reason: string) => request<USDTResolution>(`/api/v1/admin/usdt/review/${encodeURIComponent(id)}/resolution`, { method: 'PUT', body: { orderId, reason } }),
}
