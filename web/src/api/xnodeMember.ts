import type { components } from './generated'
import { request } from './http'

type Schema = components['schemas']
export type XNodeSubscription = Schema['XNodeSubscription']
export type XNodeUserMetrics = Schema['XNodeUserMetrics']
export type XNodeCustomization = Schema['XNodeCustomization']
export type RoutingChoice = Schema['XNodeRoutingChoice']
export type RoutingView = Schema['XNodeRoutingView']
export type RoutingPath = Schema['XNodeRoutingPath']
export type RouteSpeedtestWrite = Schema['XNodeRouteSpeedtestWrite']
export type RouteSpeedtestResult = Schema['XNodeRouteSpeedtestResult']
export type DIYView = Schema['XNodeDIYView']
export type DIYSplitProfile = NonNullable<DIYWrite['splitProfiles']>[number]
export type DIYSplitHost = DIYView['splitHosts'][number]
export type DIYBillingQuote = DIYView['billing']
export type DIYWhitelist = NonNullable<Schema['XNodeDIYWrite']['whitelist']>
export type DIYWhitelistView = NonNullable<DIYView['whitelist']>
export type DIYWrite = Schema['XNodeDIYWrite']
export type DIYOverrides = Schema['XNodeDIYOverrides']
export type DIYField = Schema['XNodeDIYField']
export type DIYPreview = Schema['XNodeDIYPreview']
export type DIYCompatibility = Schema['XNodeTuningApplicability']
const root = '/api/v1/xnode'
export const xnodeMemberApi = {
  diy: (signal?: AbortSignal) => request<DIYView>(`${root}/diy`, { cache: 'no-store', signal, timeoutMilliseconds: 30_000 }),
  previewDIY: (body: DIYWrite, signal?: AbortSignal) => request<DIYPreview>(`${root}/diy/preview`, { method: 'POST', body, cache: 'no-store', signal, timeoutMilliseconds: 30_000 }),
  saveDIY: (body: DIYWrite, key: string) => request<Schema['XNodeDIYSaveResult']>(`${root}/diy`, { method: 'PUT', body, headers: { 'Idempotency-Key': key }, cache: 'no-store', timeoutMilliseconds: 60_000 }),
  startRouteSpeedtest: (body: RouteSpeedtestWrite, key: string) => request<Schema['OperationReceipt']>(`${root}/routing/speedtests`, { method: 'POST', body, headers: { 'Idempotency-Key': key }, cache: 'no-store' }),
  routeSpeedtestResult: (id: string, signal?: AbortSignal) => request<RouteSpeedtestResult>(`${root}/routing/speedtests/${encodeURIComponent(id)}`, { cache: 'no-store', signal }),
  metrics: (signal?: AbortSignal) => request<XNodeUserMetrics>(`${root}/metrics`, { cache: 'no-store', signal }),
  subscription: (signal?: AbortSignal) => request<XNodeSubscription>(`${root}/subscription`, { cache: 'no-store', signal }),
  routing: (signal?: AbortSignal) => request<RoutingView>(`${root}/routing`, { cache: 'no-store', signal, timeoutMilliseconds: 30_000 }),
  previewRouting: (body: Schema['XNodeRoutingPreviewWrite'], signal?: AbortSignal) => request<Schema['XNodeRoutingPreview']>(`${root}/routing/preview`, { method: 'POST', body, cache: 'no-store', signal, timeoutMilliseconds: 30_000 }),
  saveRouting: (body: Schema['XNodeRoutingWrite'], key: string) => request<Schema['OperationReceipt']>(`${root}/routing`, { method: 'PUT', body, headers: { 'Idempotency-Key': key }, cache: 'no-store', timeoutMilliseconds: 60_000 }),
}

// Public bearer pages do not send a browser/Mini App session or request signature.
export async function publicSubscriptionMetadata(token: string, signal?: AbortSignal): Promise<XNodeSubscription> {
  const response = await fetch(`/api/v1/subscriptions/${encodeURIComponent(token)}/metadata`, { credentials: 'omit', cache: 'no-store', signal, headers: { Accept: 'application/json' } })
  if (!response.ok) throw new Error('SUBSCRIPTION_UNAVAILABLE')
  return await response.json() as XNodeSubscription
}
