import { request } from './http'
import type { components } from './generated'
import type { OperationReceipt } from './types'
type Schema = components['schemas']
export type FirewallBan = Schema['XNodeFirewallBan']
export type FirewallRecord = Schema['XNodeFirewallRecord']
export type FirewallFilter = Schema['XNodeFirewallFilter']
export type FirewallRecordSearch = Schema['XNodeFirewallRecordSearch']
export type FirewallQuote = Schema['XNodeFirewallQuote']
export type FirewallGrant = Schema['XNodeFirewallGrant']
export const firewallApi = {
  grant: (body: { prefix: string; hours: number; generation?: string; reason?: string }, key: string) => request<OperationReceipt>('/api/v1/admin/xnode/firewall/exemptions', { method: 'POST', body, headers: { 'Idempotency-Key': key }, cache: 'no-store' }),
  bans: (query = '', cursor = '', signal?: AbortSignal) => request<{ items: FirewallBan[]; nextCursor?: string }>('/api/v1/admin/xnode/firewall/bans', { query: { query, cursor, limit: 50 }, cache: 'no-store', signal }),
  records: (body: FirewallRecordSearch, signal?: AbortSignal) => request<Schema['XNodeFirewallRecordPage']>('/api/v1/admin/xnode/firewall/records/search', { method: 'POST', body, cache: 'no-store', signal }),
  quote: () => request<FirewallQuote>('/api/v1/xnode/firewall/exemption', { cache: 'no-store' }),
  purchase: (ip: string, quote: FirewallQuote, key: string) => request<FirewallGrant>('/api/v1/xnode/firewall/exemption', { method: 'POST', body: { ip, price: quote.price, hours: quote.hours }, headers: { 'Idempotency-Key': key }, cache: 'no-store' }),
}
