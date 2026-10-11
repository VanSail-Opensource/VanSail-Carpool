import { ApiError } from '@/api/http'
import type { IPLookupLiveDetails, IPLookupLiveEvent } from '@/api/ipLookup'

export function denseTopology(result: IPLookupLiveDetails) {
  const origin = 13335
  const ids = Array.from({ length: 299 }, (_, i) => 20000 + i)
  result.topology.nodes = [origin, ...ids].map((asn, i) => ({ asn, name: `Network ${asn}`, tier1: i > 20 && i <= 40 }))
  result.topology.edges = ids.map((asn, i) => ({ from: asn, to: i < 20 ? origin : ids[i % 20]!, relationship: i < 20 ? 'upstream' as const : 'observed' as const }))
  for (let i = 0; result.topology.edges.length < 1500; i++) {
    const from = ids[i % ids.length]!, to = ids[(i * 17 + 23 + Math.floor(i / ids.length)) % ids.length]!
    if (from !== to && !result.topology.edges.some(edge => edge.from === from && edge.to === to)) result.topology.edges.push({ from, to, relationship: 'observed' })
  }
  result.topology.paths = Array.from({ length: 1000 }, (_, i) => ({ asPath: [ids[i % ids.length]!, ids[i % 20]!, origin], collector: `RRC${i % 24}`, exchange: '', observedAt: result.fetchedAt }))
}

export async function publishLiveAudit(result: IPLookupLiveDetails, params: URLSearchParams, signal?: AbortSignal, emit?: (event: IPLookupLiveEvent) => void) {
  const sectionNames = ['topology', 'scores', 'ptr', 'block', 'asns', 'shodan', 'transfers'] as const
  const phases = ['topology', 'scores', 'ptr', 'block', 'asns', 'relationships', 'names', 'shodan', 'transfers'] as const
  // ?live=degraded completes the asns step with partial coverage to audit warning states.
  const degraded = params.get('live') === 'degraded'
  const tasks = (active: number) => phases.map((phase, index) => ({
    key: phase,
    status: index < active ? (degraded && phase === 'asns' ? 'partial' as const : 'success' as const) : index === active ? 'active' as const : 'pending' as const,
  }))
  for (let i = 0; i < phases.length; i++) {
    if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    emit?.({ type: 'progress', ip: result.ip, progress: { phase: phases[i]!, completed: i, total: phases.length, tasks: tasks(i) } })
    if (i < sectionNames.length) {
      const section = sectionNames[i]!
      emit?.({ type: 'section', ip: result.ip, section, data: result[section] })
    }
    if (params.get('live') === 'stream-error' && i === 2) throw new ApiError(502, { code: 'IP_DETAILS_UNAVAILABLE', message: 'Constructed stream interruption' })
    if (params.get('live') === 'slow' || degraded) await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(resolve, 350)
      signal?.addEventListener('abort', () => { clearTimeout(timer); reject(new DOMException('Aborted', 'AbortError')) }, { once: true })
    })
  }
  emit?.({ type: 'complete', ip: result.ip, data: result, progress: { phase: 'complete', completed: phases.length, total: phases.length, tasks: phases.map(phase => ({ key: phase, status: 'success' as const })) } })
}
