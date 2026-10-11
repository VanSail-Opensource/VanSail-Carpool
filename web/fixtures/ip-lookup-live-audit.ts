import { ipLookupApi, type IPLookupLiveDetails } from '@/api/ipLookup'
import { ApiError } from '@/api/http'
import { denseTopology, publishLiveAudit } from './ip-lookup-progress-audit'
import { shodanAuditDetails } from './ip-lookup-shodan-audit'

export function installLiveIPAudit(params: URLSearchParams) {
  let calls = 0
  let upstreamCalls = 0
  const cache = new Map<string, IPLookupLiveDetails>()
  const requested: string[] = []
  const fetchDetails = async (ip: string, signal?: AbortSignal, refresh = false, emit?: Parameters<typeof ipLookupApi.details>[3]): Promise<IPLookupLiveDetails> => {
    calls++; requested.push(ip)
    const stored = cache.get(ip) ?? (!ip.includes(':') ? [...cache.values()].find(value => value.ip.split('.').slice(0, 3).join('.') === ip.split('.').slice(0, 3).join('.')) : undefined)
    if (!refresh && stored) { const result = { ...structuredClone(stored), ip, sourceIP: stored.ip, cacheMatch: stored.ip === ip ? 'exact' as const : 'subnet' as const }; emit?.({ type: 'complete', ip, data: result, progress: { phase: 'complete', completed: 8, total: 8 } }); return result }
    upstreamCalls++
    if (params.get('live') === 'refresh-error' && calls > 1) throw new ApiError(502, { code: 'IP_DETAILS_UNAVAILABLE', message: 'Constructed refresh failure' })
    if (params.get('live') === 'error') throw new ApiError(429, { code: 'IP_DETAILS_RATE_LIMITED', message: 'Constructed rate limit' })
    const now = new Date().toISOString()
    const meta = { status: 'success' as const, source: 'ripe:looking-glass', observedAt: now, errorCode: '' }
    const asns = [174, 3257, 2914, 1299, 6461, 6939, 3356, 7018, 9002, 12956, 6453, 3491, 3549, 5511]
    const names: Record<number, string> = { 13335: 'Cloudflare, Inc.', 174: 'Cogent Communications', 3257: 'GTT Communications', 2914: 'NTT DATA', 1299: 'Arelion', 6461: 'Zayo', 6939: 'Hurricane Electric', 3356: 'Lumen', 7018: 'AT&T', 9002: 'RETN', 12956: 'Telefónica', 6453: 'Tata Communications', 3491: 'PCCW Global', 3549: 'Level 3', 5511: 'Orange', 701: 'Verizon' }
    const result: IPLookupLiveDetails = {
      ip, sourceIP: ip, cacheMatch: 'none', fetchedAt: now,
      topology: { ...meta, prefix: ip.includes(':') ? '2606:4700:4700::/48' : '8.8.8.0/24', origins: [13335], nodes: [13335, ...asns, 48266, 701].map(asn => ({ asn, name: names[asn] ?? 'Catixs' })), edges: [...asns.map((asn, index) => ({ from: asn, to: 13335, relationship: index < 4 ? 'upstream' as const : 'observed' as const })), { from: 48266, to: 174, relationship: 'observed' }], paths: asns.map((asn, index) => ({ asPath: index === 0 ? [48266, asn, 13335, 13335] : [asn, 13335], collector: `RRC${String(index).padStart(2, '0')}`, exchange: index === 0 ? 'LINX / LONAP' : '', observedAt: now })), truncated: false, relationshipMeta: { ...meta, source: 'caida', observedAt: '2026-10-01' } },
      asns: [{ asn: 13335, name: names[13335]!, registeredAt: '2010-07-14T00:00:00Z', rir: 'ARIN', announced: true, ipv4Prefixes: 2676, ipv6Prefixes: 3049, observedNeighbors: 1543, peers: 362, upstreams: 109, sources: { registration: { ...meta, source: 'ripe:rir' }, rir: { ...meta, source: 'ripe:rir' }, routing: { ...meta, source: 'ripe:routing-status' }, relationships: { ...meta, source: 'caida', observedAt: '2026-10-01' } }, traffic: {
        botHuman: { ...meta, source: 'cloudflare', values: { bot: 32.7, human: 67.3 }, startTime: '2026-10-03', endTime: '2026-10-10' },
        devices: { ...meta, source: 'cloudflare', values: { desktop: 42.3, mobile: 55.8, other: 1.9 }, startTime: '2026-10-03', endTime: '2026-10-10' },
        ipVersion: { ...meta, source: 'cloudflare', values: { IPv4: 58.4, IPv6: 41.6 }, startTime: '2026-10-03', endTime: '2026-10-10' },
      } }],
      scores: { ...meta, source: 'ipapi', company: { ratio: 0.0003, label: 'Low' }, asn: { ratio: 0, label: 'Very Low' } },
      block: { ...meta, source: 'abuseipdb', prefix: '8.8.8.0/24', reportedAddresses: 0, addressCapacity: '256', windowDays: 30 },
      ptr: { ...meta, source: 'dns', domains: ['dns.google', 'resolver.google'] },
      transfers: { ...meta, source: 'ripe:transfer-history', transfers: [{ sourceResource: '8.8.0.0/16', recipientResource: '8.8.8.0/24', sourceHolderName: 'Previous network operator', recipientHolderName: 'Current network operator', sourceRegistry: 'ARIN', recipientRegistry: 'RIPE NCC', transferTime: '2025-08-12T00:00:00Z', transferType: 'POLICY' }] },
      shodan: { ...meta, source: 'shodan:internetdb', observedAt: '', hostnames: ['dns.google'], ports: [53, 443, 853] },
    }
    result.shodan = shodanAuditDetails(params, result.shodan)
    for (const asn of result.asns) { asn.abuseContacts = ['abuse@cloudflare.com']; asn.website = 'https://www.cloudflare.com'; asn.sources.abuseContacts = { ...meta, source: 'ripe:abuse-contact-finder' }; asn.sources.website = { ...meta, source: 'peeringdb' } }
    result.topology.paths.push({ asPath: [701, 48266, 174, 13335], collector: 'RRC21', exchange: '', observedAt: now })
    if (params.get('live') === 'empty') {
      result.topology = { ...result.topology, status: 'empty', prefix: '', origins: [], nodes: [], edges: [], paths: [] }
      result.transfers = { ...result.transfers!, status: 'empty', transfers: [] }
      result.asns = []; result.ptr = { ...result.ptr, status: 'empty', domains: [] }
      result.shodan = { ...result.shodan, status: 'empty', hostnames: [], ports: [], cpes: [], tags: [], vulns: [] }
      result.scores = { ...result.scores, status: 'unconfigured', company: { ratio: null, label: '' }, asn: { ratio: null, label: '' } }
      result.block = { ...result.block, status: 'empty', reportedAddresses: null, addressCapacity: '' }
    }
    if (params.get('live') === 'partial') {
      result.block = { ...result.block, status: 'restricted', reportedAddresses: null, errorCode: 'IP_DETAILS_PLAN_RESTRICTED' }
      result.ptr = { ...result.ptr, status: 'error', domains: [], errorCode: 'IP_DETAILS_UNAVAILABLE' }
      result.shodan = { ...result.shodan, status: 'error', hostnames: [], ports: [], cpes: [], tags: [], vulns: [], errorCode: 'IP_DETAILS_UNAVAILABLE' }
      for (const summary of Object.values(result.asns[0]!.traffic)) { summary.status = 'unconfigured'; summary.values = {} }
    }
    if (params.get('live') === 'moas') {
      result.topology.origins.push(48266)
      result.topology.paths.push({ asPath: [3257, 48266], collector: 'RRC21', exchange: '', observedAt: now })
      result.topology.edges.push({ from: 3257, to: 48266, relationship: 'upstream' })
      result.asns.push({ ...result.asns[0]!, asn: 48266, name: 'Catixs', rir: 'RIPE NCC' })
    }
    result.topology.classificationMeta = { ...meta, source: 'caida:clique', observedAt: '2026-10-01' }
    const tiers = new Set([701, 174, 3257, 2914, 1299, 6461, 3356, 7018, 12956, 6453, 3491, 5511])
    result.topology.nodes = result.topology.nodes.map(node => ({ ...node, tier1: params.get('live') === 'unknown-t1' ? null : tiers.has(node.asn) }))
    if (params.get('live') === 'unknown-t1') result.topology.classificationMeta.status = 'partial'
    if (params.get('live') === 'transits') {
      const paths = [[701, 888, 999, 174, 38195, 9280, 45671, 45671], [3257, 38195, 9280, 45671], [24482, 58511, 9280, 45671], [24482, 58511, 9280, 45671]]
      result.topology.origins = [45671]; result.topology.prefix = '118.127.59.0/24'
      result.topology.nodes = [...new Set(paths.flat())].map(asn => ({ asn, name: asn === 45671 ? 'Servers Australia' : `Network ${asn}`, tier1: tiers.has(asn) }))
      const pairs = new Map<string, { from: number; to: number; relationship: 'upstream' }>()
      for (const path of paths) path.forEach((asn, i) => { if (i > 0 && asn !== path[i - 1]) pairs.set(`${path[i - 1]}:${asn}`, { from: path[i - 1]!, to: asn, relationship: 'upstream' }) })
      result.topology.edges = [...pairs.values()]
      result.topology.paths = paths.map(asPath => ({ asPath, collector: 'RRC01', exchange: '', observedAt: now }))
      result.asns = [{ ...result.asns[0]!, asn: 45671, name: 'Servers Australia' }]
    }
    if (params.get('live') === 'large') denseTopology(result)
    if (params.get('live') === 'transfer-error') result.transfers = { ...result.transfers!, status: 'error', transfers: [], errorCode: 'IP_DETAILS_UNAVAILABLE' }
    result.block = { ...result.block, prefix: ip.includes(':') ? '2606:4700:4700::/64' : `${ip.split('.').slice(0, 3).join('.')}.0/24`, addressCapacity: ip.includes(':') ? '18446744073709551616' : '256' }
    compactAuditTopology(result)
    if (params.get('live') === 'legacy' && !refresh) { delete result.transfers; delete result.topology.projectionVersion; delete result.topology.routeSummaries; for (const asn of result.asns) { delete asn.abuseContacts; delete asn.website; delete asn.sources.abuseContacts; delete asn.sources.website }; result.block = { ...result.block, prefix: '', addressCapacity: '', reportedAddresses: null, status: 'empty', errorCode: 'IP_DETAILS_LEGACY' } }
    await publishLiveAudit(result, params, signal, params.has('baseline') ? undefined : emit)
    cache.set(ip, structuredClone(result))
    return result
  }
  ipLookupApi.details = (ip, signal, _refresh, emit) => fetchDetails(ip, signal, false, emit)
  return { counters: () => ({ calls, requested, upstreamCalls }), refresh: (ip: string) => fetchDetails(ip, undefined, true), seed: (details: IPLookupLiveDetails) => cache.set(details.ip, structuredClone(details)) }
}

function compactAuditTopology(result: IPLookupLiveDetails) {
 const t = result.topology
 const tiers = new Set(t.nodes.filter(node => node.tier1).map(node => node.asn))
 const kept = new Set(t.origins), counts = new Map<number, number>()
 const paths = t.paths.map(route => [...route.asPath].reverse().filter((asn, i, path) => i === 0 || asn !== path[i - 1]))
 for (const path of paths) {
  path.forEach((asn, i) => { if (i === 0 || i <= 3 && !tiers.has(path[0]!) || tiers.has(asn)) kept.add(asn) })
  for (const asn of new Set(path)) counts.set(asn, (counts.get(asn) ?? 0) + 1)
 }
 t.nodes = t.nodes.filter(node => kept.has(node.asn)).map(node => ({ ...node, observedRoutes: counts.get(node.asn) ?? 0 }))
 t.edges = t.edges.filter(edge => kept.has(edge.from) && kept.has(edge.to)).map(edge => ({ ...edge, observedRoutes: paths.filter(path => path.some((asn, i) => i > 0 && asn === edge.from && path[i - 1] === edge.to)).length, relationshipMeta: t.relationshipMeta }))
 t.routeSummaries = paths.map(path => {
  const indices = path.flatMap((asn, i) => kept.has(asn) && (!tiers.has(path[0]!) || i === 0 || tiers.has(asn)) ? [i] : [])
  return { asns: indices.map(i => path[i]!), hops: indices.slice(1).map((value, i) => value - indices[i]!), observedRoutes: 1 }
 })
 t.projectionVersion = 2; t.paths = []; t.tier1Links = []
}
