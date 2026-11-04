import type { components } from './generated'
import type { DeepReadonly } from './types'
import { request } from './http'
export type PersonalForecast = DeepReadonly<components['schemas']['PersonalForecast']>
export type UserPricePage = DeepReadonly<components['schemas']['UserPricePage']>
export const forecastApi = {
  user: (id: string, signal?: AbortSignal) => request<PersonalForecast>(`/api/v1/admin/users/${encodeURIComponent(id)}/predictions`, { signal, cache: 'no-store' }),
  prices: (id: string, cursor?: string, signal?: AbortSignal) => request<UserPricePage>(`/api/v1/admin/users/${encodeURIComponent(id)}/prices`, { signal, query: { cursor, limit: 100 }, cache: 'no-store' }),
}
