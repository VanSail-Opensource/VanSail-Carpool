import type { components } from './generated'
import { request } from './http'
import type { DeepReadonly } from './types'

export type UptimeTimeline = DeepReadonly<components['schemas']['UptimeTimeline']>
export type UptimeSummary = DeepReadonly<components['schemas']['UptimeSummary']>
export type SubscriptionHost = DeepReadonly<components['schemas']['SubscriptionHost']>
export type SubscriptionSquad = DeepReadonly<components['schemas']['SubscriptionSquad']>
export type MemberSubscription = DeepReadonly<components['schemas']['MemberSubscription']>

export const subscriptionApi = {
  saveHostName: (id: string, name: string) => request<components['schemas']['HostNameCustomization']>(`/api/v1/subscription/hosts/${encodeURIComponent(id)}/name`, { method: 'PUT', body: { name } }),
  summary: (signal?: AbortSignal) => request<UptimeSummary>('/api/v1/connectivity/summary', { signal, cache: 'no-store' }),
  subscription: (signal?: AbortSignal) => request<MemberSubscription>('/api/v1/subscription', { signal, cache: 'no-store' }),
}
