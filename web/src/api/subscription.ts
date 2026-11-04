import type { components } from './generated'
import { request } from './http'
import type { DeepReadonly } from './types'

export type SubscriptionHost = DeepReadonly<components['schemas']['SubscriptionHost']>
export type MemberSubscription = DeepReadonly<components['schemas']['MemberSubscription']>

export const subscriptionApi = {
  saveHostName: (id: string, name: string) => request<components['schemas']['HostNameCustomization']>(`/api/v1/subscription/hosts/${encodeURIComponent(id)}/name`, { method: 'PUT', body: { name } }),
  subscription: (signal?: AbortSignal) => request<MemberSubscription>('/api/v1/subscription', { signal, cache: 'no-store' }),
}
