import { firewallApi, type AutomaticLeasePage } from '@/api/firewall'
import { ApiError } from '@/api/http'

export function automaticLeaseFixture(params: URLSearchParams) {
  const state = { mode: params.get('lease-state') ?? 'populated', calls: [] as { query: string; page: number }[], delay: 0 }
  firewallApi.leases = async (query = '', page = 1, signal) => {
    state.calls.push({ query, page })
    await new Promise(resolve => setTimeout(resolve, state.delay || (state.mode === 'loading' ? 3000 : 50)))
    if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
    if (state.mode === 'error') throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
    const now = Date.now(), at = (seconds: number) => new Date(now + seconds * 1000).toISOString()
    const items: AutomaticLeasePage['items'] = state.mode === 'empty' ? [] : [
      { ip: page === 1 ? '8.8.8.8' : '9.9.9.9', firstActive: at(-86400), lastActive: at(-4), leaseStarted: at(-3600), renewAfter: at(82800), renewedAt: at(-3600), expiresAt: at(169200), remainingSeconds: 169200, observationCount: '9007199254740993', renewalCount: '3', sourceCount: 8, sourceCountTruncated: true, sourceNodeIds: Array.from({ length: 8 }, (_, i) => `historical-node-${i + 1}`), pendingNodes: 2, banReleasePending: true },
      { ip: '2606:4700:4700::1111', firstActive: at(-60), lastActive: at(-3), leaseStarted: at(-60), renewAfter: at(43140), renewedAt: null, expiresAt: at(86340), remainingSeconds: 86340, observationCount: '12', renewalCount: '0', sourceCount: 1, sourceCountTruncated: false, sourceNodeIds: ['historical-node-1'], pendingNodes: 0, banReleasePending: false },
    ]
    const matching = items.filter(item => item.ip.includes(query))
    return { items: matching, total: query || state.mode === 'empty' ? matching.length : 75, page, pageSize: 25, statistics: { active: 10000, ipv4: 7000, ipv6: 3000, created: '9007199254740993', renewed: '3456', expired: '321', rejected: '2' }, settings: { enabled: true, durationHours: 24 }, fetchedAt: at(0) }
  }
  Object.assign(globalThis, { __leaseFixture: state })
}
