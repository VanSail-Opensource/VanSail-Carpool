import type { components } from './generated'
import { ApiError, request } from './http'
import { withRequestDeadline } from './requestDeadline'
import { isResponseCompatible } from './contracts/responses'

type Schema = components['schemas']
export type XNodeSubscription = Schema['XNodeSubscription']
export type XNodeUserMetrics = Schema['XNodeUserMetrics']
export type RoutingChoice = Schema['XNodeRoutingChoice']
export type RoutingView = Schema['XNodeRoutingView']
export type RoutingPath = Schema['XNodeRoutingPath']
export type RouteSpeedtestWrite = Schema['XNodeRouteSpeedtestWrite']
export type RouteSpeedtestResult = Schema['XNodeRouteSpeedtestResult']
export type DIYView = Schema['XNodeDIYView']
export type DIYSplitProfile = NonNullable<DIYWrite['splitProfiles']>[number]
export type DIYSplitHost = DIYView['splitHosts'][number]
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
  const path = `/api/v1/subscriptions/${encodeURIComponent(token)}/metadata`
  return withRequestDeadline(30_000, signal, async (requestSignal) => {
    const response = await fetch(path, {
      credentials: 'omit', cache: 'no-store', signal: requestSignal, headers: { Accept: 'application/json' },
    })
    // Preserve status for bearer recovery without invoking session-expiry events.
    if (!response.ok) throw new ApiError(response.status, { code: 'SUBSCRIPTION_UNAVAILABLE', message: 'SUBSCRIPTION_UNAVAILABLE' })
    const payload: unknown = await response.json()
    if (!isResponseCompatible(path, 'GET', payload)) throw new ApiError(502, { code: 'SUBSCRIPTION_UNAVAILABLE', message: 'SUBSCRIPTION_UNAVAILABLE' })
    return payload as XNodeSubscription
  })
}
