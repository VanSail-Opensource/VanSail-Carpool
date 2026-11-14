import type { components } from './generated'
import { request } from './http'

export type UserPreferences = components['schemas']['UserPreferences']
export type UserPreferencesPatch = components['schemas']['UserPreferencesPatch']
export type GroupMemberTag = components['schemas']['GroupMemberTag']

export const preferencesApi = {
  get: () => request<UserPreferences>('/api/v1/me/preferences', { cache: 'no-cache' }),
  update: (body: UserPreferencesPatch) => request<UserPreferences>('/api/v1/me/preferences', { method: 'PATCH', body }),
  getTag: () => request<GroupMemberTag>('/api/v1/me/group-member-tag', { cache: 'no-cache' }),
  updateTag: (tag: string) => request<GroupMemberTag>('/api/v1/me/group-member-tag', { method: 'PUT', body: { tag } }),
}
