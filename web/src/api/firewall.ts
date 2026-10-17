import { request } from './http'
import type { components } from './generated'
type Schema = components['schemas']
export type FirewallBan = Schema['XNodeFirewallBan']
export type FirewallQuote = Schema['XNodeFirewallQuote']
export type FirewallGrant = Schema['XNodeFirewallGrant']
export const firewallApi = {
  bans: () => request<{ items: FirewallBan[] }>('/api/v1/admin/xnode/firewall/bans', { cache: 'no-store' }),
  quote: () => request<FirewallQuote>('/api/v1/xnode/firewall/exemption', { cache: 'no-store' }),
  purchase: (ip: string, quote: FirewallQuote, key: string) => request<FirewallGrant>('/api/v1/xnode/firewall/exemption', { method: 'POST', body: { ip, price: quote.price, hours: quote.hours }, headers: { 'Idempotency-Key': key }, cache: 'no-store' }),
}
