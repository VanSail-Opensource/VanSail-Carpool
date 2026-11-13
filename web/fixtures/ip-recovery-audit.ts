import '@/styles/main.css'
import { createApp, defineComponent, h, shallowRef } from 'vue'
import { createPinia } from 'pinia'
import ui from '@nuxt/ui/vue-plugin'
import { ipLookupApi, type IPLookupBGPTopology, type IPLookupLiveDetails, type IPLookupReport } from '@/api/ipLookup'
import type { Session } from '@/api/types'
import { useSessionStore } from '@/stores/session'
import { setLocale, t } from '@/i18n'
import { setExperience } from '@/utils/experience'
import IPRecoveryAudit from './IPRecoveryAudit.vue'

setExperience('miniapp')
setLocale('en')
const now = new Date().toISOString()
const meta = { status: 'success' as const, source: 'ripe', observedAt: now, errorCode: '' }
const routeEnds = Array.from({ length: 30 }, (_, i) => 64600 + i)
const topology = shallowRef<IPLookupBGPTopology>({
  ...meta, prefix: '192.0.2.0/24', origins: [64500], projectionVersion: 2,
  nodes: [{ asn: 64500, name: 'Origin' }, { asn: 64501, name: 'Transit' }, ...routeEnds.map(asn => ({ asn, name: `Network ${asn}` }))],
  edges: [{ from: 64501, to: 64500, relationship: 'upstream' },
    ...routeEnds.map(asn => ({ from: asn, to: 64501, relationship: 'upstream' as const })),
    { from: 64999, to: 64500, relationship: 'upstream' }],
  routeSummaries: [...routeEnds.map(asn => ({ asns: [64500, 64501, asn], hops: [1, 1], observedRoutes: 1 })),
    { asns: [64500, 64999], hops: [1], observedRoutes: 1 }],
  paths: [], truncated: false, relationshipMeta: meta,
})
const report: IPLookupReport = {
  id: 'constructed-report', ip: '192.0.2.1', status: 'succeeded', refundRequired: false, verdict: 'suitable', reasons: [],
  facts: { country: '', city: '', latitude: null, longitude: null, asn: '', asnName: '', networkType: '' },
  maxmind: { ip_risk_snapshot: null, static_ip_score: null, user_count: null, user_type: null },
  sources: {}, refusals: [], databases: [], checkedAt: now, policyVersion: 'constructed', parserVersion: 'constructed',
}
function details(domain: string): IPLookupLiveDetails {
  return { ip: report.ip, sourceIP: report.ip, cacheMatch: 'none', fetchedAt: now,
    topology: { ...meta, status: 'empty', prefix: '', origins: [], nodes: [], edges: [], paths: [], truncated: false, relationshipMeta: meta },
    asns: [], scores: { ...meta, company: { ratio: null, label: '' }, asn: { ratio: null, label: '' } },
    ptr: { ...meta, domains: [domain] }, shodan: { ...meta, hostnames: [], ports: [] },
  }
}
const snapshot = details('previous-owner.example')
const pending: Array<(value: IPLookupLiveDetails) => void> = []
let reads = 0
ipLookupApi.details = async () => { reads++; return new Promise(resolve => { pending.push(resolve) }) }
const pinia = createPinia()
const session = useSessionStore(pinia)
function owner(id: string): Session {
  return { authenticated: true, user: { id, telegramId: '42', firstName: 'Audit', lastName: '', telegramUsername: '',
    displayName: 'Audit', username: null, role: 'user', onboardingState: 'complete', groupJoined: true, channelJoined: true,
    policyAcceptedAt: now, agreementRevision: 1, recoveryReason: '', displayCurrency: 'TXB', createdAt: now, updatedAt: now } }
}
// This component fixture owns its reads; full-session preloads are audited elsewhere.
session.session = owner('initial-owner')
session.status = 'ready'
const live = new URLSearchParams(location.search).has('live')
const Root = defineComponent({ setup: () => () => h(IPRecoveryAudit, { topology: topology.value, report, snapshot, live }) })
const app = createApp(Root).use(pinia).use(ui)
app.config.globalProperties.$t = t
Object.assign(window, { __ipRecoveryAudit: {
  shrink: () => { topology.value = { ...topology.value, routeSummaries: topology.value.routeSummaries!.slice(0, 1) } },
  removeTransit: () => { topology.value = { ...topology.value, nodes: topology.value.nodes.filter(node => node.asn !== 64501) } },
  replaceOwner: () => { session.session = owner('replacement-owner') },
  logout: () => session.clear(),
  release: () => pending.shift()?.(details('retired-read.example')),
  reads: () => reads,
} })
app.mount('#app')
