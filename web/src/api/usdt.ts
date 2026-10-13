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
  config: () => request<USDTConfig>('/api/v1/admin/usdt', { cache: 'no-store' }),
  save: (body: USDTConfig) => request<USDTConfig>('/api/v1/admin/usdt', { method: 'PUT', body }),
  status: () => request<USDTStatus>('/api/v1/admin/usdt/status', { cache: 'no-store' }),
  check: (network: USDTNetworkID) => request<components['schemas']['USDTCheckResult']>('/api/v1/admin/usdt/check', { method: 'POST', body: { network } }),
  review: (cursor?: string) => request<USDTReviewPage>('/api/v1/admin/usdt/review', { cache: 'no-store', query: { cursor, limit: 50 } }),
  resolve: (id: string, orderId: string, reason: string) => request<USDTResolution>(`/api/v1/admin/usdt/review/${encodeURIComponent(id)}/resolution`, { method: 'PUT', body: { orderId, reason } }),
}
