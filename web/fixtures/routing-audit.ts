import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { xnodeApi } from '@/api/xnode'
import { xnodeMemberApi, type RoutingChoice, type RoutingPath, type RoutingView } from '@/api/xnodeMember'
import { memberOperationsApi } from '@/api/memberOperations'
import { ApiError } from '@/api/http'
import { initializeBrowserTheme } from '@/browser/theme'
import { operationReceiptLoaderKey } from '@/composables/operationReceiptLoader'
import { setExperience } from '@/utils/experience'
import { setLocale, t } from '@/i18n'
import type { components } from '@/api/generated'
import type { OperationReceipt } from '@/api/types'
import RoutingAudit from './RoutingAudit.vue'
import { installDIYAudit } from './diy-audit'

const params = new URLSearchParams(location.search)
setExperience(params.has('admin') ? 'browser' : 'miniapp')
if (params.has('admin')) { await import('@/browser/styles.css'); initializeBrowserTheme() }
else { await import('@/styles/main.css') }
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const trace = { calls: [] as string[], writes: [] as unknown[], keys: [] as string[], previews: [] as unknown[], polls: 0 }
const nodes = params.has('empty') ? [] : [{ id: 'a', name: 'Amsterdam' }, { id: 'b', name: 'Singapore' }, { id: 'c', name: 'Tokyo' }]
if (params.has('unused') && nodes.length) nodes.push({ id: 'd', name: 'New York' })
const endpoints: NonNullable<RoutingView['endpoints']> = nodes.flatMap(node => [
  { id: `${node.id}-primary`, nodeId: node.id, name: `${node.name} primary`, address: `${node.id}.proxy.example`, port: 443 },
  { id: `${node.id}-secondary`, nodeId: node.id, name: `${node.name} alternate`, address: `${node.id}-alt.proxy.example`, port: 8443 },
])
if (params.has('long-names')) for (const endpoint of endpoints) endpoint.name += ' dedicated international connection for member routing'
let catalog: components['schemas']['XNodeRoutingTemplateConfig'] = { revision: 1, templates: params.has('no-templates') ? [] : [{ id: 'google-ai', name: 'Google + AI', match: { domain: ['domain:google.com', 'domain:openai.com'] } }, { id: 'google', name: 'Google', match: { domain: ['domain:google.com'] } }, { id: 'ai', name: 'AI', match: { domain: ['domain:openai.com'] } }] }
if (params.has('geosite')) for (const template of catalog.templates) if (template.id !== 'ai') template.match.domain = ['geosite:google']
if (params.has('unrelated-geosite')) catalog.templates.push({ id: 'unrelated', name: 'Unused streaming', match: { domain: ['geosite:youtube'] } })
let plan: RoutingView['plan'] = { revision: 1, phase: params.has('disabled') ? 'disabled' : 'active', reason: params.has('disabled') ? 'routing_legacy_incompatible' : '', choices: params.has('empty') || params.has('no-routes') ? [] : [{ id: 'ab', inboundNodeId: 'a', outboundNodeId: 'b', templateId: 'google-ai' }, { id: 'bc', inboundNodeId: 'b', outboundNodeId: 'c', templateId: 'google' }] }
let failed = false, receipt: OperationReceipt | null = null
function pause(delay = params.has('slow') ? 2000 : 30) { return new Promise(resolve => setTimeout(resolve, delay)) }
function path(ids: string[], rules: string[]): RoutingPath { return { nodes: ids, rules, conditions: [], exclusions: [], excludedRules: ids.join() === 'a,b' ? ['bc'] : [] } }
function graph(choices: RoutingChoice[]) {
  const result: RoutingPath[] = []
  for (const start of nodes) for (const domain of ['google.com', 'openai.com']) {
    const ids = [start.id], rules: string[] = []
    for (let step = 0; step < 10; step++) {
      const rule = choices.find(rule => { const domains = catalog.templates.find(template => template.id === rule.templateId)?.match.domain ?? []; return rule.inboundNodeId === ids.at(-1) && (domains.includes(`domain:${domain}`) || domain === 'google.com' && domains.includes('geosite:google')) })
      if (!rule) break
      if (ids.includes(rule.outboundNodeId)) throw new ApiError(422, { code: 'routing_loop', message: '', details: { rules: [...rules, rule.id].join(',') } })
      ids.push(rule.outboundNodeId); rules.push(rule.id)
    }
    if (rules.length && !result.some(p => p.nodes.join() === ids.join())) result.push(path(ids, rules))
  }
  return result
}
xnodeMemberApi.routing = async () => { trace.calls.push('member-load'); await pause(); if (params.has('load-error') && !failed) { failed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }; return structuredClone({ plan, nodes, endpoints, templates: catalog.templates, paths: graph(plan.choices) }) }
xnodeMemberApi.previewRouting = async body => {
  trace.calls.push('member-preview'); trace.previews.push(JSON.parse(JSON.stringify(body))); await pause(params.has('stale-preview') && body.destination.domain === 'google.com' ? 1700 : 50)
  const selectedGeoSite = body.choices.some(choice => catalog.templates.find(template => template.id === choice.templateId)?.match.domain?.some(domain => domain.startsWith('geosite:')))
  if (selectedGeoSite && params.has('geosite-missing')) throw new ApiError(422, { code: 'XNODE_GEOSITE_REQUIRED', message: '' })
  if (selectedGeoSite && params.has('geosite-unavailable')) throw new ApiError(503, { code: 'XNODE_GEOSITE_UNAVAILABLE', message: '' })
  if (params.has('preview-error')) throw new ApiError(422, { code: 'routing_overlap_unproven', message: '' })
  const paths = graph(body.choices), domain = body.destination.domain ?? '', exact = domain.endsWith('google.com') ? ['a', 'b', 'c'] : domain.endsWith('openai.com') ? ['a', 'b'] : [body.start]
  return { paths, trace: body.start ? path(exact.slice(Math.max(0, exact.indexOf(body.start))), exact.length === 3 ? ['ab', 'bc'] : ['ab']) : null }
}
xnodeMemberApi.saveRouting = async (body, key) => {
  trace.calls.push('member-save'); trace.keys.push(key); await pause()
  if (params.has('save-error')) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' })
  trace.writes.push(JSON.parse(JSON.stringify(body))); plan = { ...plan, revision: body.revision + 1, choices: JSON.parse(JSON.stringify(body.choices)), phase: 'applying', operationId: 'routing-operation' }
  receipt = { id: 'routing-operation', kind: 'xnode_routing', status: 'queued', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }; return receipt
}
memberOperationsApi.getOperation = async () => { trace.polls++; trace.calls.push('member-operation'); await pause(30); if (params.has('poll-error')) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }); plan = { ...plan, phase: 'active' }; return { ...receipt!, status: 'succeeded' } }
xnodeApi.routingTemplates = async () => { trace.calls.push('admin-load'); await pause(); if (params.has('load-error') && !failed) { failed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }; return structuredClone(catalog) }
xnodeApi.saveRoutingTemplates = async value => { trace.calls.push('admin-save'); await pause(); if (params.has('save-error')) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' }); trace.writes.push(JSON.parse(JSON.stringify(value))); catalog = JSON.parse(JSON.stringify({ ...value, revision: value.revision + 1 })); return structuredClone(catalog) }
Object.assign(window, { __routingAudit: trace })
let speedtestPolls = 0
const speedtestRequests = new Map<string, OperationReceipt>()
xnodeMemberApi.startRouteSpeedtest = async (body, key) => {
  trace.calls.push('speedtest-start'); trace.keys.push(key); await pause()
  if (!speedtestRequests.has(key)) {
    trace.writes.push(JSON.parse(JSON.stringify(body)))
    speedtestRequests.set(key, { id: 'speedtest-operation', kind: 'xnode_route_speedtest', status: 'queued', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() })
    if (params.has('speedtest-start-error')) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
  }
  return speedtestRequests.get(key)!
}
xnodeMemberApi.routeSpeedtestResult = async () => {
  trace.calls.push('speedtest-result'); speedtestPolls++; await pause(50)
  if (params.has('speedtest-poll-error') && speedtestPolls === 1) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
  const done = speedtestPolls >= 3
  const uncertain = done && params.has('speedtest-uncertain')
  const receipt: OperationReceipt = { ...[...speedtestRequests.values()][0]!, status: done ? uncertain ? 'pending_review' : 'succeeded' : 'processing' }
  const first = { sourceNodeId: 'a', targetEndpointId: 'b-primary', status: speedtestPolls > 1 ? 'succeeded' as const : 'processing' as const, ...(speedtestPolls > 1 ? { measurement: { request_latency_millis: [3, 5, 4], download_mbps: 20, upload_mbps: 8 } } : {}) }
  const second = { sourceNodeId: 'b', targetEndpointId: 'c-primary', status: done ? uncertain ? 'pending_review' as const : 'succeeded' as const : 'queued' as const, ...(done && !uncertain ? { measurement: { request_latency_millis: [11, 9, 10], download_mbps: 12, upload_mbps: 15 } } : {}) }
  return { receipt, pairs: [first, second], ...(done && !uncertain ? { estimate: { latencyMillis: 14, downloadMbps: 12, uploadMbps: 8 } } : {}) }
}
if (params.has('diy')) installDIYAudit(params, trace, () => xnodeMemberApi.routing())
const app = createApp(RoutingAudit); app.config.globalProperties.$t = t
app.provide(operationReceiptLoaderKey, memberOperationsApi.getOperation)
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: RoutingAudit }] })).use(ui).mount('#app')
