import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { xnodeApi, type XNode, type XNodeGlobal, type XNodeMigration, type XNodeConfiguration, type SubscriptionTemplateConfig } from '@/api/xnode'
import type { OperationReceipt } from '@/api/types'
import { memberOperationsApi } from '@/api/memberOperations'
import { ApiError } from '@/api/http'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import XNodeAudit from './XNodeAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser'); initializeBrowserTheme(); setLocale(params.has('zh') ? 'zh-CN' : 'en')
const trace = { calls: [] as string[], copies: [] as string[], reloads: 0 }
const stamp = '2026-10-15T00:00:00Z'
const nodes: XNode[] = params.has('empty') ? [] : [
  { id: '11111111-1111-4111-8111-111111111111', name: 'Amsterdam', countryCode: 'NL', providerName: 'Fixture', multiplier: 1.25, enabled: true, configId: 'shared', agentId: 'fixture-agent-one', desiredRevision: 3, acceptedRevision: 3, appliedRevision: 3 },
  { id: '22222222-2222-4222-8222-222222222222', name: 'Singapore', countryCode: 'SG', providerName: 'Fixture', multiplier: 1, enabled: true, configId: 'shared', desiredRevision: 0, acceptedRevision: 0, appliedRevision: 0 },
]
const configurations: XNodeConfiguration[] = params.has('empty') ? [] : [{ id: 'shared', name: 'Shared configuration', revision: 1, snippetIds: [], document: { inbounds: [{ tag: 'paid', port: 443, protocol: 'vless', settings: { decryption: 'none' } }], outbounds: [{ tag: 'direct', protocol: 'freedom' }, { UserOutbound: {} }], routing: { rules: [{ UserRouting: {} }] } } }]
let global: XNodeGlobal = { connectionLimit: 0, blockedPorts: '', torrent: { enabled: false }, telemetry: { sample_seconds: 1, report_seconds: 5, include_active_ips: false }, updates: { enabled: false }, providers: {} }
let migration: XNodeMigration = { finalDigest: '', finalBackupReference: '', finalizedAt: null, phase: params.has('source') ? 'none' : 'active', snapshotDigest: params.has('source') ? '' : 'a'.repeat(64), sourceBackupReference: '', backupName: params.has('source') ? '' : 'verified-panel-backup.db', nodes: params.has('source') ? [] : nodes.map((node, i) => ({ nodeId: node.id, legacyNodeId: node.id, phase: params.has('review') && i === 0 ? 'review' : params.has('final') || i === 0 ? 'native' : 'legacy', sourceDigest: 'a'.repeat(64), usageCutoff: i === 0 ? stamp : null, errorCode: params.has('review') && i === 0 ? 'XNODE_ROLLBACK_PENDING' : '' })) }
let latest: OperationReceipt | null = null
const pause = async () => { if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 2000)) }
async function check(name: string): Promise<void> {
  trace.calls.push(name); await pause()
  if (params.has('error') && trace.reloads++ === 0) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
}
function receipt(kind: string): OperationReceipt { latest = { id: crypto.randomUUID(), kind, status: 'queued', createdAt: stamp, updatedAt: stamp }; return latest }
xnodeApi.nodes = async () => { await check('nodes'); return { items: structuredClone(nodes) } }
xnodeApi.node = async id => ({ node: nodes.find(node => node.id === id)!, settings: { ddns: [], certificates: { primary: { provider: '', config: { domain: 'proxy.example', cert_mode: 'http', auto_tls: true } } }, inboundCertificates: {} } })
xnodeApi.configurations = async () => ({ items: structuredClone(configurations) })
xnodeApi.snippets = async () => ({ items: [] })
xnodeApi.groups = async () => ({ items: params.has('empty') ? [] : [{ id: 'group-one', name: 'Global group', enabled: true, bindings: nodes.map(node => ({ nodeId: node.id, inboundTag: 'paid' })) }] })
xnodeApi.endpoints = async () => ({ items: params.has('empty') ? [] : nodes.map(node => ({ id: node.id, nodeId: node.id, inboundTag: 'paid', name: node.name, address: `${node.countryCode?.toLowerCase()}.proxy.example`, port: 443, enabled: true, hidden: false, overrides: {} })) })
xnodeApi.metrics = async () => ({ nodes: params.has('empty') ? [] : nodes.map((node, i) => ({ nodeId: node.id, connected: i === 0, ready: i === 0, synchronized: i === 0, stale: i !== 0, version: 'v1.0.2', xrayVersion: '26.10.10', capabilities: ['traffic_periods_v1', 'traffic_period_barriers_v1'], reportedAt: stamp, machine: { cpu_usage: [12, 18], memory: { total: '8589934592', used: '2147483648' }, host: { hostname: node.name, platform: 'debian', platformVersion: '13' }, errors: {} }, certificates: {}, activeConnections: i === 0 ? 8 : 0, usersOnline: i === 0 ? 2 : 0, uploadBytesPerSecond: i === 0 ? 250000 : 0, downloadBytesPerSecond: i === 0 ? 1500000 : 0, logs: [] })), onlineNodes: nodes.length ? 1 : 0, totalNodes: nodes.length, activeConnections: nodes.length ? 8 : 0, uploadBytesPerSecond: 250000, downloadBytesPerSecond: 1500000, fetchedAt: stamp })

let templateConfig: SubscriptionTemplateConfig = { revision: 1, templates: params.has('empty') ? [] : [
  { id: 'xray-default', name: 'Xray default', format: 'xray', document: { inbounds: [], outbounds: [{ SubscriptionProxies: {} }, { tag: 'direct', protocol: 'freedom' }], routing: { rules: [] } } },
  { id: 'singbox-default', name: 'sing-box default', format: 'sing-box', document: { outbounds: [{ SubscriptionProxies: {} }] } },
  { id: 'clash-default', name: 'Clash Meta default', format: 'clash', document: { proxies: [{ SubscriptionProxies: {} }], 'proxy-groups': [], rules: [] } },
], defaults: params.has('empty') ? {} : { xray: 'xray-default', 'sing-box': 'singbox-default', clash: 'clash-default' }, assignments: {}, formats: { native: true, xray: true, 'sing-box': true, clash: true } }
let templateLoadFailed = false
xnodeApi.subscriptionTemplates = async () => { await check('templates'); if (params.has('template-load-error') && !templateLoadFailed) { templateLoadFailed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }; return JSON.parse(JSON.stringify(templateConfig)) as SubscriptionTemplateConfig }
xnodeApi.saveSubscriptionTemplates = async value => { await check('save-templates'); if (params.has('template-conflict')) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' }); templateConfig = JSON.parse(JSON.stringify({ ...value, revision: value.revision + 1 })) as SubscriptionTemplateConfig; return templateConfig }
xnodeApi.previewSubscriptionTemplate = async value => { await check('preview-template'); return { validation: { valid: !params.has('template-invalid'), needsAgentValidation: false, errorCode: '', detail: params.has('template-invalid') ? 'selector references an unknown proxy' : '' }, preview: value.format === 'clash' ? 'proxies:\n  - name: Preview\n    type: vless' : JSON.stringify(value.format === 'xray' ? [{ remarks: 'Preview', outbounds: [{ protocol: 'vless', tag: 'proxy-preview' }] }] : { outbounds: [{ type: 'vless', tag: 'Preview' }] }) } }

xnodeApi.global = async () => global
xnodeApi.saveGlobal = async value => { trace.calls.push('save-global'); global = value; return global }
xnodeApi.enroll = async id => { trace.calls.push(`enroll:${id}`); return { masterUrl: 'wss://panel.example/v1/agents/connect', token: 'fixture-token', protocol: 'x-node.v1' } }
xnodeApi.validate = async () => params.has('invalid') ? { valid: false, needsAgentValidation: false, errorCode: 'XNODE_CONFIG_INVALID', detail: 'outbound[1]: unsupported protocol' } : { valid: true, needsAgentValidation: false, errorCode: '' }
xnodeApi.command = async (_id, body) => { trace.calls.push('command:' + body.name); return receipt('xnode_command') }
xnodeApi.commandResult = async () => ({ receipt: { ...latest!, status: 'succeeded' }, result: { command: 'speedtest', request_latency_millis: 31, download_mbps: 842.5, upload_mbps: 522.7, download_bytes: '67108864', upload_bytes: '33554432' } })
xnodeApi.migration = async () => { await check('migration'); return structuredClone(migration) }
xnodeApi.previewMigration = async () => { trace.calls.push('preview'); migration = { ...migration, phase: 'preview', snapshotDigest: 'b'.repeat(64), sourceBackupReference: 'verified-source-backup' }; return { digest: migration.snapshotDigest, nodes: nodes.length, groups: 1, accounts: 12, endpoints: nodes.length, issues: params.has('shared') ? [{ resourceId: '33333333-3333-4333-8333-333333333333', code: 'SHARED_SS2022_HOST', blocking: true }] : params.has('utility') ? [{ resourceId: 'service-profile', code: 'UNSUPPORTED_UTILITY_INBOUND', blocking: true }] : params.has('issue') ? [{ resourceId: 'legacy-ss', code: 'UNSUPPORTED_SHADOWSOCKS', blocking: true }] : [] } }
xnodeApi.importMigration = async () => { trace.calls.push('import'); migration = { ...migration, phase: 'active', backupName: 'verified-panel-backup.db', nodes: nodes.map(node => ({ nodeId: node.id, legacyNodeId: node.id, phase: 'legacy', sourceDigest: migration.snapshotDigest, usageCutoff: null, errorCode: '' })) }; return receipt('xnode_migration') }
xnodeApi.migrateNode = async (id, action) => { trace.calls.push(`${action}:${id}`); if (params.has('shared-error')) return receipt('xnode_migration'); migration = { ...migration, nodes: migration.nodes.map(node => node.nodeId === id ? { ...node, phase: action === 'cutover' ? 'native' : 'legacy', errorCode: '' } : node) }; return receipt('xnode_migration') }
xnodeApi.stageFinalMigration = async reference => { await check('final-export'); migration = { ...migration, finalDigest: 'c'.repeat(64), finalBackupReference: reference }; return receipt('xnode_migration') }
xnodeApi.finalizeMigration = async digest => { await check('finalize:' + digest); if (params.has('final-error')) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' }); migration = { ...migration, phase: 'complete', finalizedAt: stamp }; return receipt('xnode_migration') }
memberOperationsApi.getOperation = async () => ({ ...latest!, status: params.has('shared-error') ? 'failed' : params.has('pending') ? 'pending_review' : 'succeeded', errorCode: params.has('shared-error') ? 'XNODE_SHARED_ENDPOINT' : null, updatedAt: new Date().toISOString() })
Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { trace.copies.push(value) } } })
Object.assign(window, { __xnodeAudit: trace })
const app = createApp(XNodeAudit)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: XNodeAudit }] })).use(ui).mount('#app')
