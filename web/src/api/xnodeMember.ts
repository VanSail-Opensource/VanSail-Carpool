import type { components } from './generated'
import { request } from './http'

type Schema = components['schemas']
export type XNodeSubscription = Schema['XNodeSubscription']
export type XNodeUserMetrics = Schema['XNodeUserMetrics']
export type XNodeCustomization = Schema['XNodeCustomization']
const root = '/api/v1/xnode'
export const xnodeMemberApi = {
  metrics: () => request<XNodeUserMetrics>(`${root}/metrics`, { cache: 'no-store' }),
  subscription: () => request<XNodeSubscription>(`${root}/subscription`, { cache: 'no-store' }),
  routing: (id: string) => request<XNodeCustomization>(`${root}/routing/${encodeURIComponent(id)}`, { cache: 'no-store' }),
  templates: () => request<Schema['XNodeRoutingTemplates']>(`${root}/routing/templates`, { cache: 'no-store' }),
  saveRouting: (id: string, kind: 'outbounds' | 'rules', body: Schema['XNodeCustomizationWrite']) => request<XNodeCustomization>(`${root}/routing/${encodeURIComponent(id)}/${kind}`, { method: 'PUT', body, cache: 'no-store' }),
}

// Public bearer pages do not send a browser/Mini App session or request signature.
export async function publicSubscriptionMetadata(token: string, signal?: AbortSignal): Promise<XNodeSubscription> {
  const response = await fetch(`/api/v1/subscriptions/${encodeURIComponent(token)}/metadata`, { credentials: 'omit', cache: 'no-store', signal, headers: { Accept: 'application/json' } })
  if (!response.ok) throw new Error('SUBSCRIPTION_UNAVAILABLE')
  return await response.json() as XNodeSubscription
}
