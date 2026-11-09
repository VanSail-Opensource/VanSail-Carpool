import type { DeepReadonly, OperationReceipt, SquadProduct } from './types'
import type { components } from './generated'
import { request } from './http'

export type HourlyPolicy = components['schemas']['HourlyPolicy']
export type HourlyPolicySnapshot = components['schemas']['HourlyPolicySnapshot']
export type HourlyOffer = components['schemas']['HourlyOffer']
export type HourlyEnrollment = components['schemas']['HourlyEnrollment']
export type HourlyExtraPreference = components['schemas']['HourlyExtraPreference']
export type HourlyAccountUsage = components['schemas']['HourlyAccountUsage']
export type HourlySquadView = Omit<components['schemas']['HourlySquadView'], 'squad'> & { squad: SquadProduct }
export type HourlyAccountView = Omit<components['schemas']['HourlyAccountView'], 'squads'> & { squads: HourlySquadView[] }
export type HourlyView = DeepReadonly<HourlyAccountView>
export type HourlySquad = DeepReadonly<HourlySquadView>

export const hourlyApi = {
  get: (signal?: AbortSignal) => request<HourlyView>('/api/v1/hourly', { cache: 'no-store', signal }),
  activate: (groupId: string, policyVersion: string, offerRevision: string, key: string, activationCode?: string) =>
    request<OperationReceipt>('/api/v1/hourly/activate', {
      method: 'POST', headers: { 'Idempotency-Key': key },
      body: { groupId, policyVersion, offerRevision, ...(activationCode ? { activationCode } : {}) },
    }),
  disable: (id: string, policyVersion: string, key: string) =>
    request<OperationReceipt>(`/api/v1/hourly/enrollments/${encodeURIComponent(id)}/disable`, {
      method: 'POST', headers: { 'Idempotency-Key': key }, body: { policyVersion },
    }),
  extra: (enabled: boolean, policyVersion: string, acceptedPriceTXBMinor?: string) =>
    request<HourlyExtraPreference>('/api/v1/hourly/extra-traffic', {
      method: 'PUT', body: { enabled, policyVersion, ...(enabled ? { acceptedPriceTXBMinor } : {}) },
    }),
  policy: () => request<HourlyPolicySnapshot>('/api/v1/admin/hourly-policy', { cache: 'no-store' }),
  savePolicy: (version: string, policy: HourlyPolicy) =>
    request<HourlyPolicySnapshot>('/api/v1/admin/hourly-policy', { method: 'PUT', body: { version, policy } }),
  offers: () => request<HourlyOffer[]>('/api/v1/admin/hourly-offers', { cache: 'no-store' }),
  saveOffer: (groupId: string, revision: string, enabled: boolean, hourlyPriceTXBMinor: string, requiredHours: number) =>
    request<HourlyOffer>(`/api/v1/admin/hourly-offers/${encodeURIComponent(groupId)}`, {
      method: 'PUT', body: { revision, enabled, hourlyPriceTXBMinor, requiredHours },
    }),
}
