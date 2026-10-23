import { firewallApi } from '@/api/firewall'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { xnodeApi, type XNode, type XNodeGlobal, type XNodeConfiguration, type SubscriptionTemplateConfig } from '@/api/xnode'
import type { OperationReceipt } from '@/api/types'
import { adminOperationsApi } from '@/api/adminOperations'
import { operationReceiptLoaderKey } from '@/composables/operationReceiptLoader'
import { ApiError } from '@/api/http'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import { subscriptionPreset } from '@/components/admin/xnode/subscriptionTemplatePresets'
import XNodeAudit from './XNodeAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser'); initializeBrowserTheme(); setLocale(params.has('zh') ? 'zh-CN' : 'en')
const trace = { calls: [] as string[], copies: [] as string[], writes: [] as unknown[], reloads: 0 }
globalThis.addEventListener('catalog-quota-save', event => trace.writes.push((event as CustomEvent).detail))
const stamp = '2026-10-15T00:00:00Z'
const nodes: XNode[] = params.has('empty') ? [] : [
  { id: '11111111-1111-4111-8111-111111111111', name: 'Amsterdam', countryCode: 'NL', providerName: 'Fixture', multiplier: 1.25, enabled: true, configId: 'shared', agentId: 'fixture-agent-one', desiredRevision: 3, acceptedRevision: 3, appliedRevision: 3 },
  { id: '22222222-2222-4222-8222-222222222222', name: 'Singapore', countryCode: 'SG', providerName: 'Fixture', multiplier: 1, enabled: true, configId: 'shared', desiredRevision: 0, acceptedRevision: 0, appliedRevision: 0 },
]
const configurations: XNodeConfiguration[] = params.has('empty') ? [] : [{ id: 'shared', name: 'Shared configuration', revision: 1, snippetIds: [], document: { inbounds: [{ tag: 'paid', port: 443, protocol: 'vless', settings: { decryption: 'none' } }], outbounds: [{ tag: 'direct', protocol: 'freedom' }, { UserOutbound: {} }], routing: { rules: [{ UserRouting: {} }] } } }]
let global: XNodeGlobal = { geodataIntervalHours: params.has('legacy-schedule') ? undefined : 24, geodataCron: params.has('legacy-schedule') ? '0 * * * *' : '', sourceGeoIp: { enabled: false, selectors: ['geoip:cn'] }, blockedDomains: [], connectionLimit: 0, blockedPorts: '', torrent: { enabled: false }, telemetry: { sample_seconds: 1, report_seconds: 5, include_active_ips: false }, updates: { enabled: false }, providers: {} }
let latest: OperationReceipt | null = null
const pause = async () => { if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 2000)) }
async function check(name: string): Promise<void> {
  trace.calls.push(name); await pause()
  if (params.has('error') && trace.reloads++ === 0) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
}
function receipt(kind: string): OperationReceipt { latest = { id: crypto.randomUUID(), kind, status: 'queued', createdAt: stamp, updatedAt: stamp }; return latest }
xnodeApi.nodes = async () => { await check('nodes'); return { items: structuredClone(nodes) } }
let zoneReads = 0
xnodeApi.providerZones = async (id, signal) => {
  trace.calls.push('zones:' + id); zoneReads++
  if (params.has('zones-slow')) await new Promise(resolve => setTimeout(resolve, 3000))
  if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
  if (params.has('zones-error') && zoneReads === 1) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
  return { supported: id !== 'other', items: params.has('zones-empty') || id === 'other' ? [] : [{ id: 'zone-one', name: 'proxy.example' }, { id: 'zone-two', name: 'vansail.example' }] }
}
let nodeLoadFailed = false
xnodeApi.node = async id => {
  await check('node:' + id)
  if (params.has('node-load-error') && !nodeLoadFailed) { nodeLoadFailed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
  if (params.has('stale-node')) await new Promise(resolve => setTimeout(resolve, id === nodes[0]?.id ? 1500 : 30))
  return { node: nodes.find(node => node.id === id)!, settings: { ddns: [], certificates: params.has('no-certificates') ? {} : { primary: { provider: '', config: params.has('node-file') ? { cert_mode: 'file', cert_file: '/etc/x-node/tls/cert.pem', key_file: '/etc/x-node/tls/key.pem' } : { domain: 'proxy.example', cert_mode: 'http', auto_tls: true } } }, inboundCertificates: {}, dns: { mode: 'xray', redirect: false, upstreams: [id === nodes[0]?.id ? '1.1.1.1' : '8.8.8.8'] }, firewall: { enabled: false, protect: false, tcp_ports: [443], ssh_ports: [22], allow_prefixes: [], udp_ports: [] } } }
}
xnodeApi.saveNode = async (node, settings) => {
  await check('save-node')
  if (params.has('node-save-error')) throw new ApiError(422, { code: 'XNODE_INPUT_INVALID', message: '' })
  trace.writes.push(JSON.parse(JSON.stringify({ node, settings })))
  return { ...nodes[0]!, ...node, id: node.id || 'fixture-created-node' }
}
xnodeApi.configurations = async () => ({ items: structuredClone(configurations) })
xnodeApi.snippets = async () => ({ items: [] })
xnodeApi.groups = async () => ({ items: params.has('empty') ? [] : [{ id: 'group-one', name: 'Global group', enabled: true, bindings: nodes.map(node => ({ nodeId: node.id, inboundTag: 'paid' })) }] })
const deletedEndpoints = new Set<string>()
xnodeApi.deleteEndpoint = async id => { await check('delete-endpoint:' + id); if (params.has('delete-error')) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' }); deletedEndpoints.add(id); trace.writes.push({ deleteEndpoint: id }) }
xnodeApi.endpoints = async () => ({ items: params.has('empty') ? [] : nodes.filter(node => !deletedEndpoints.has(node.id)).map(node => ({ id: node.id, nodeId: node.id, inboundTag: 'paid', name: node.name, address: `${node.countryCode?.toLowerCase()}.proxy.example`, port: 443, enabled: true, hidden: false, overrides: {} })) })
xnodeApi.saveGroup = async value => { await check('save-group'); trace.writes.push(JSON.parse(JSON.stringify(value))); return { id: value.id || 'fixture-group', name: value.name || '', enabled: value.enabled ?? true, bindings: value.bindings ?? [] } }
xnodeApi.saveEndpoint = async value => { await check('save-endpoint'); trace.writes.push(JSON.parse(JSON.stringify(value))); return { id: value.id || 'fixture-endpoint', nodeId: value.nodeId || '', inboundTag: value.inboundTag || '', name: value.name || '', address: value.address || '', port: value.port || 443, enabled: value.enabled ?? true, hidden: value.hidden ?? false, overrides: value.overrides ?? {} } }
let logReads = 0
xnodeApi.logs = async id => {
 await check('logs:' + id); logReads++
 if (params.has('log-error') && logReads === 1) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
 if (params.has('stale-logs')) await new Promise(resolve => setTimeout(resolve, id === nodes[0]?.id ? 1600 : 30))
 return { nodeId: id, connected: id === nodes[0]?.id, stale: id !== nodes[0]?.id, entries: params.has('log-empty') ? [] : [{ time: new Date().toISOString(), source: 'xray', text: `Constructed ${nodes.find(node => node.id === id)?.name} log update ${logReads}` }], dropped: params.has('log-dropped') ? '9007199254740993' : '0', fetchedAt: new Date().toISOString() }
}
xnodeApi.metrics = async () => {
  await check('metrics')
  const count = nodes.length ? 1 : 0
  return { nodes: params.has('empty') ? [] : nodes.map((node, i) => ({ nodeId: node.id, available: i === 0 && !params.has('metrics-unavailable'), connected: i === 0, ready: i === 0 && !params.has('not-ready'), synchronized: i === 0 && !params.has('unsynchronized'), stale: i !== 0 || params.has('stale-metrics'), version: 'v1.0.2', xrayVersion: '26.10.10', capabilities: ['traffic_periods_v1', 'traffic_period_barriers_v1'], reportedAt: stamp, machine: { ...(params.has('large-metrics') ? { cpu_info: Array.from({ length: 64 }, (_, cpu) => ({ cpu, flags: Array.from({ length: 100 }, (_, flag) => `flag_${flag}`) })) } : {}), cpu_usage: [12, 18], memory: { total: '8589934592', used: '2147483648' }, host: { hostname: node.name, platform: 'debian', platformVersion: '13' }, errors: {} }, certificates: {}, activeConnections: i === 0 ? 8 : 0, usersOnline: i === 0 ? 2 : 0, uploadBytesPerSecond: i === 0 ? 250000 : 0, downloadBytesPerSecond: i === 0 ? 1500000 : 0, logs: [] })), onlineNodes: count, totalNodes: nodes.length, activeConnections: count ? 8 : 0, uploadBytesPerSecond: count ? 250000 : 0, downloadBytesPerSecond: count ? 1500000 : 0, fetchedAt: stamp }
}

let templateConfig: SubscriptionTemplateConfig = { revision: 1, templates: params.has('empty') ? [] : [
  { id: 'xray-default', name: 'Xray default', format: 'xray', document: { inbounds: [], outbounds: [{ SubscriptionProxies: {} }, { tag: 'direct', protocol: 'freedom' }], routing: { rules: [] } } },
  { id: 'singbox-default', name: 'sing-box default', format: 'sing-box', document: { outbounds: [{ SubscriptionProxies: {} }] } },
  { id: 'stash-default', name: 'Stash default', format: 'stash', document: subscriptionPreset('stash') },
  { id: 'clash-default', name: 'Clash Meta default', format: 'clash', document: { proxies: [{ SubscriptionProxies: {} }], 'proxy-groups': [], rules: [] } },
], defaults: params.has('empty') ? {} : { xray: 'xray-default', 'sing-box': 'singbox-default', clash: 'clash-default', stash: 'stash-default' }, userAgentRules: params.has('empty') ? [] : [{ pattern: '(?i)stash', templateId: 'stash-default' }], assignments: { 'retained-member': { xray: 'xray-default' } }, formats: { base64: true, stash: true, native: true, xray: true, 'sing-box': true, clash: true } }
let templateLoadFailed = false
xnodeApi.subscriptionTemplates = async () => { await check('templates'); if (params.has('template-load-error') && !templateLoadFailed) { templateLoadFailed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }; return JSON.parse(JSON.stringify(templateConfig)) as SubscriptionTemplateConfig }
xnodeApi.saveSubscriptionTemplates = async value => { await check('save-templates'); if (params.has('template-conflict')) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' }); trace.writes.push(JSON.parse(JSON.stringify(value))); templateConfig = JSON.parse(JSON.stringify({ ...value, revision: value.revision + 1 })) as SubscriptionTemplateConfig; return templateConfig }
xnodeApi.previewSubscriptionTemplate = async (value, signal) => {
  await check('preview-template')
  if (params.has('template-invalid')) return { validation: { valid: false, needsAgentValidation: false, errorCode: 'XNODE_CONFIG_INVALID', detail: 'selector references an unknown proxy' }, preview: '' }
  const response = await fetch('http://127.0.0.1:5175/template', { method: 'POST', body: JSON.stringify(value), headers: { 'Content-Type': 'application/json' }, signal })
  if (!response.ok) throw new ApiError(response.status, { code: 'XNODE_INPUT_INVALID', message: '' })
  return response.json()
}

xnodeApi.previewSubscriptionSelection = async (body, signal) => {
  await check('preview-selection')
  const response = await fetch('http://127.0.0.1:5175/selection', { method: 'POST', body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' }, signal })
  if (!response.ok) throw new ApiError(response.status, { code: 'XNODE_INPUT_INVALID', message: '' })
  return response.json()
}

global.firewallDetection = { rules: Object.fromEntries(['closed_port', 'invalid_packet', 'syn_flood', 'icmp_probe', 'xray_tcp_probe', 'xray_invalid', 'xray_auth_failure'].map(name => [name, { enabled: true, threshold: 3, window_seconds: 86400 }])), banSeconds: 259200, migrationNotice: params.has('migration-notice') }
if (params.has('legacy-source-rule')) global.firewallDetection.rules.source_match = { enabled: true, threshold: 3, window_seconds: 86400 }
let globalLoadFailed = false
function globalProjection() { const value = JSON.parse(JSON.stringify(global)) as XNodeGlobal; for (const provider of Object.values(value.providers)) for (const key of Object.keys(provider.credentials)) provider.credentials[key] = ''; return value }
xnodeApi.global = async () => { await check('global'); if (params.has('global-load-error') && !globalLoadFailed) { globalLoadFailed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }; return globalProjection() }
xnodeApi.saveGlobal = async value => {
  await check('save-global')
  const policyCode = params.get('policy-error')
  if (policyCode && ['XNODE_DOMAIN_SELECTOR_INVALID', 'XNODE_GEOSITE_REQUIRED', 'XNODE_GEOSITE_UNAVAILABLE', 'XNODE_GEOIP_INVALID', 'XNODE_GEOIP_UNAVAILABLE'].includes(policyCode)) throw new ApiError(422, { code: policyCode, message: '' })
  if (params.has('global-save-error')) throw new ApiError(422, { code: 'XNODE_INPUT_INVALID', message: '' })
  trace.writes.push(JSON.parse(JSON.stringify(value))); global = JSON.parse(JSON.stringify(value)) as XNodeGlobal; return globalProjection()
}
xnodeApi.enroll = async id => { trace.calls.push(`enroll:${id}`); return { masterUrl: 'wss://panel.example/v1/agents/connect', token: 'fixture-token', protocol: 'x-node.v1' } }
xnodeApi.validate = async () => params.has('invalid') ? { valid: false, needsAgentValidation: false, errorCode: 'XNODE_CONFIG_INVALID', detail: 'outbound[1]: unsupported protocol' } : { valid: true, needsAgentValidation: false, errorCode: '' }
xnodeApi.command = async (_id, body) => { trace.calls.push('command:' + body.name); trace.writes.push({ command: structuredClone(body) }); await check('command'); return receipt('xnode_command') }
xnodeApi.commandResult = async () => ({ receipt: { ...latest!, status: 'succeeded' }, result: { command: 'speedtest', request_latency_millis: 31, download_mbps: 842.5, upload_mbps: 522.7, download_bytes: '67108864', upload_bytes: '33554432' } })
adminOperationsApi.getOperation = async () => { trace.calls.push('admin-operation'); return ({ ...latest!, status: params.has('command-outcome') ? (params.get('command-outcome') === 'OUTCOME_UNKNOWN' ? 'pending_review' : 'failed') : params.has('shared-error') ? 'failed' : params.has('pending') ? 'pending_review' : 'succeeded', errorCode: params.has('command-outcome') ? 'XNODE_COMMAND_' + params.get('command-outcome') : params.has('shared-error') ? 'XNODE_SHARED_ENDPOINT' : null, updatedAt: new Date().toISOString() }) }
firewallApi.grant = async (body, key) => { await check('grant-firewall'); if (params.has('grant-error')) throw new ApiError(422, { code: 'XNODE_INPUT_INVALID', message: '' }); trace.writes.push({ body, key }); return receipt('xnode_firewall_exemption') }
Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { trace.copies.push(value) } } })
Object.assign(window, { __xnodeAudit: trace })
if (params.has('responses')) {
  templateConfig.templates.push({ id: 'empty-response', name: 'Limited client response', format: 'empty-http', statusCode: 451 })
  templateConfig.userAgentRules = [{ pattern: 'Browser', action: 'browser', templateId: '' }, { pattern: 'Blocked', templateId: 'empty-response' }]
}
if (params.has('ddns')) {
  global.providers = { cloudflare: { type: 'cloudflare', credentials: {} }, other: { type: 'duckdns', credentials: {} } }
  const originalNode = xnodeApi.node
  xnodeApi.node = async (id, signal) => {
    const value = await originalNode(id, signal)
    value.settings.ddns = [{ id: 'primary', provider: 'cloudflare', zone: 'saved.example', name: 'node', ttl_seconds: 300 }]
    return value
  }
}

let serviceUsers = params.has('services-empty') ? [] : [{ identity: { proxyUserId: '1099511627776', userId: '', username: 'routing-service', serviceName: 'routing-service', currentPeriodId: 'service-period', accessDisabled: false, createdAt: stamp }, access: { limitBytes: '0', strategy: 'NO_RESET' as const, startedAt: stamp, expiresAt: '2027-10-17T00:00:00Z', groupIds: ['group-one'], disabled: false } }]
xnodeApi.namedServices = async () => { await check('services'); if (params.has('service-error')) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }); return { items: structuredClone(serviceUsers) } }
xnodeApi.saveNamedService = async (id, body) => {
  await check('save-service')
  if (params.has('service-save-error')) throw new ApiError(422, { code: 'XNODE_INPUT_INVALID', message: '' })
  const item = { identity: { ...serviceUsers[0]?.identity, proxyUserId: id || '1099511627777', userId: '', username: body.name, serviceName: body.name, currentPeriodId: 'service-period', accessDisabled: body.access.disabled, createdAt: stamp }, access: JSON.parse(JSON.stringify(body.access)) }
  serviceUsers = [...serviceUsers.filter(value => value.identity.proxyUserId !== id), item]
  return item
}
xnodeApi.serviceSubscription = async () => ({ username: 'routing-service', url: 'https://panel.example/subscription/fixture-only', downloadUrl: 'https://panel.example/api/v1/subscriptions/fixture-only', usedBytes: '400', limitBytes: '1000', active: true, incomplete: false, formats: { native: true }, expiresAt: '2027-10-17T00:00:00Z' })
xnodeApi.rotateServiceSubscription = async () => { trace.calls.push('rotate-service'); return { url: 'https://panel.example/subscription/rotated-fixture-only' } }
xnodeApi.serviceRouting = async () => { await check('service-routing'); return { userId: '', nodeId: nodes[0]!.id, proxyUserId: '1099511627776', revision: 0, outbounds: [], routing: [] } }
xnodeApi.saveServiceRouting = async (_id, _node, part, body) => { trace.calls.push('save-service-' + part); return { ...body, revision: body.revision + 1 } }
const releasedBans = new Set<string>()
let pendingRelease = '', releasePolls = 0
const originalGrant = firewallApi.grant, originalOperation = adminOperationsApi.getOperation
firewallApi.grant = async (body, key) => { const result = await originalGrant(body, key); if (body.hours === 0) { pendingRelease = body.prefix; releasePolls = 0 }; return result }
adminOperationsApi.getOperation = async id => {
  const result = await originalOperation(id)
  if (pendingRelease && result.kind === 'xnode_firewall_exemption') {
    if (params.has('firewall-release-pending') && releasePolls++ < 2) return { ...result, status: 'processing' }
    releasedBans.add(pendingRelease); pendingRelease = ''
  }
  return result
}
firewallApi.bans = async (query = '', cursor = '') => {
  await check('firewall-bans:' + query + ':' + cursor)
  if (params.has('firewall-error')) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
  if (query === 'invalid') throw new ApiError(422, { code: 'XNODE_INPUT_INVALID', message: '' })
  const base = { generation: 'fixture-ban', rule: 'xray_tcp_probe', count: '7', detectedAt: stamp, until: '2027-10-20T00:00:00Z', pendingNodes: 2, refresh_seconds: 259200 }
  const first = { ...base, prefix: '2001:4860:1234:5678::/64', ip: '2001:4860:1234:5678::8' }
  const second = { ...base, prefix: '8.8.8.8/32', ip: '8.8.8.8', generation: 'second-ban' }
  const selected = query.startsWith('8.8') || cursor ? second : first
  return { items: params.has('firewall-empty') || releasedBans.has(selected.prefix) || query.startsWith('9.9') ? [] : [selected], ...(params.has('firewall-pages') && !cursor && !query ? { nextCursor: 'second' } : {}) }
}
firewallApi.quote = async () => { await check('firewall-quote'); if (params.has('quote-error')) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }); return { enabled: !params.has('unban-disabled') && !params.has('whitelist'), whitelistMode: params.has('whitelist'), price: '1.25', hours: 24 } }
firewallApi.purchase = async (ip, quote, key) => { await check('firewall-purchase:' + ip + ':' + key); if (params.has('unban-error')) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' }); return { id: key, ip, priceMinor: '125', hours: quote.hours, until: '2026-10-20T00:00:00Z' } }
const app = createApp(XNodeAudit)
app.config.globalProperties.$t = t
app.provide(operationReceiptLoaderKey, adminOperationsApi.getOperation)
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: XNodeAudit }] })).use(ui).mount('#app')
