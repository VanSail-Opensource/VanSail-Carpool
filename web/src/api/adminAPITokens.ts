import type { components } from './generated'
import { request } from './http'

type Schema = components['schemas']
export type PanelAPIToken = Schema['PanelAPIToken']
export type PanelAPITokenWrite = Schema['PanelAPITokenWrite']
export type PanelAPITokenIssued = Schema['PanelAPITokenIssued']
export const apiTokenScopes = ['admin:read', 'admin:write', 'xnode:read', 'xnode:write', 'migration:read', 'migration:write'] as const

const root = '/api/v1/admin/api-tokens'
export const adminAPITokensApi = {
  list: () => request<{ items: PanelAPIToken[] }>(root, { cache: 'no-store' }),
  create: (body: PanelAPITokenWrite) => request<PanelAPITokenIssued>(root, { method: 'POST', body, cache: 'no-store' }),
  revoke: (id: string) => request<void>(`${root}/${encodeURIComponent(id)}`, { method: 'DELETE', cache: 'no-store' }),
}
