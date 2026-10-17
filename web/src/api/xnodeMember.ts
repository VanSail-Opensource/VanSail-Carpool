import type { components } from './generated'
import { request } from './http'

type Schema = components['schemas']
export type XNodeSubscription = Schema['XNodeSubscription']
export type XNodeUserMetrics = Schema['XNodeUserMetrics']
export type XNodeCustomization = Schema['XNodeCustomization']
export type RoutingChoice = Schema['XNodeRoutingChoice']
export type RoutingView = Schema['XNodeRoutingView']
export type RoutingPath = Schema['XNodeRoutingPath']
const root = '/api/v1/xnode'
export const xnodeMemberApi = {
  metrics: (signal?: AbortSignal) => request<XNodeUserMetrics>(`${root}/metrics`, { cache: 'no-store', signal }),
  subscription: (signal?: AbortSignal) => request<XNodeSubscription>(`${root}/subscription`, { cache: 'no-store', signal }),
  routing: (signal?: AbortSignal) => request<RoutingView>(`${root}/routing`, { cache: 'no-store', signal }),
  previewRouting: (body: Schema['XNodeRoutingPreviewWrite'], signal?: AbortSignal) => request<Schema['XNodeRoutingPreview']>(`${root}/routing/preview`, { method: 'POST', body, cache: 'no-store', signal }),
  saveRouting: (body: Schema['XNodeRoutingWrite'], key: string) => request<Schema['OperationReceipt']>(`${root}/routing`, { method: 'PUT', body, headers: { 'Idempotency-Key': key }, cache: 'no-store' }),
}

// Public bearer pages do not send a browser/Mini App session or request signature.
export async function publicSubscriptionMetadata(token: string, signal?: AbortSignal): Promise<XNodeSubscription> {
  const response = await fetch(`/api/v1/subscriptions/${encodeURIComponent(token)}/metadata`, { credentials: 'omit', cache: 'no-store', signal, headers: { Accept: 'application/json' } })
  if (!response.ok) throw new Error('SUBSCRIPTION_UNAVAILABLE')
  return await response.json() as XNodeSubscription
}
