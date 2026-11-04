import { adminOperationsApi, type XNodeProfileMetrics } from '@/api/adminOperations'
import { ApiError } from '@/api/http'

export function profileMetricsFixture(params: URLSearchParams) {
  const state = { mode: params.get('profile-state') ?? 'populated', calls: [] as string[], delay: 0 }
  adminOperationsApi.getUserMetrics = async (user, signal) => {
    state.calls.push(user)
    await new Promise(resolve => setTimeout(resolve, state.delay || (state.mode === 'loading' ? 3000 : 50)))
    if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
    if (state.mode === 'error') throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
    const now = Date.now(), at = (ms: number) => new Date(now + ms).toISOString(), hour = (offset: number) => new Date(Math.floor(now / 3600000) * 3600000 + offset * 3600000).toISOString()
    const base = { countryCode: 'NL', available: true, currentUse: true, connections: '2', uploadBytesPerSecond: 12345, downloadBytesPerSecond: 54321, uploadBytes: '9007199254740993', downloadBytes: '7', weightedBytes: '9007199254791000', nativeWeightedBytes: '9007199254741000', activeIpStatus: 'available' as const, sampledAt: at(-1000), activeIpsTruncated: params.has('profile-truncated') }
    const nodes: XNodeProfileMetrics['nodes'] = state.mode === 'empty' ? [] : [
      { ...base, nodeId: 'current-node', name: 'Amsterdam', activeIps: [{ ip: '8.8.8.8', lastActive: at(-2000), observations: '9007199254740993' }, { ip: '2606:4700:4700::1111', lastActive: at(-10000), observations: '5' }, { ip: '10.0.0.8', lastActive: at(-20000), observations: '2' }] },
      { ...base, nodeId: 'quiet-node', name: 'Tokyo', countryCode: 'JP', currentUse: false, connections: '0', uploadBytesPerSecond: 0, downloadBytesPerSecond: 0, activeIps: [] },
      { ...base, nodeId: 'stale-node', name: 'Singapore', countryCode: 'SG', available: false, activeIpStatus: 'stale', sampledAt: at(-120000), activeIps: [] },
      { ...base, nodeId: 'legacy-node', name: 'Unavailable', countryCode: '', available: false, activeIpStatus: 'unavailable', sampledAt: null, activeIps: [] },
    ]
    return {
      connections: nodes.length ? 2 : 0, uploadBytesPerSecond: nodes.length ? 12345 : 0, downloadBytesPerSecond: nodes.length ? 54321 : 0, incomplete: Boolean(nodes.length), limited: false, reasons: [], fetchedAt: at(0), usedTrafficBytes: '9007199254791000', lifetimeTrafficBytes: '900719925474099300', nodes,
      usage: Array.from({ length: 7 }, (_, i) => ({ date: at((i - 6) * 86400000).slice(0, 10), uploadBytes: state.mode === 'empty' ? '0' : '9007199254740993', downloadBytes: state.mode === 'empty' ? '0' : '7', weightedBytes: '9007199254791000', nativeWeightedBytes: state.mode === 'empty' ? '0' : '9007199254741000', legacyBytes: '50000' })),
      paygHistory: {
        from: new Date(Math.floor((now - 72 * 3600000) / 60000) * 60000).toISOString(), to: at(0), truncated: params.has('profile-truncated'),
        nodes: state.mode === 'empty' ? [] : [{ nodeId: 'current-node', name: 'Amsterdam', countryCode: 'NL', uploadBytes: '9007199254740993', downloadBytes: '7' }, { nodeId: 'removed-node', name: 'Retired Frankfurt', countryCode: 'DE', uploadBytes: '11', downloadBytes: '13' }],
        hours: state.mode === 'empty' ? [] : [{ nodeId: 'current-node', hour: hour(-1), uploadBytes: '9007199254740993', downloadBytes: '7' }, { nodeId: 'removed-node', hour: hour(-1), uploadBytes: '11', downloadBytes: '13' }],
      },
    }
  }
  Object.assign(globalThis, { __profileFixture: state })
}
