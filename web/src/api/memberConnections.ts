import type { components } from './generated'
import { request } from './http'

export type MemberConnection = components['schemas']['MemberConnection']
export type MemberConnectionPage = components['schemas']['MemberConnectionPage']
const root = '/api/v1/subscription/connections'
export const memberConnectionsApi = {
  list: (search: string, page: number, signal?: AbortSignal) => request<MemberConnectionPage>(root, { query: { search, page, pageSize: 25 }, cache: 'no-store', signal }),
  grant: (ip: string) => request<MemberConnection>(`${root}/leases`, { method: 'POST', body: { ip }, cache: 'no-store' }),
}
