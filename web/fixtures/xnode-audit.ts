import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { xnodeApi, type XNode, type XNodeGlobal, type XNodeMigration, type XNodeConfiguration } from '@/api/xnode'
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
let migration: XNodeMigration = { phase: params.has('source') ? 'none' : 'active', snapshotDigest: params.has('source') ? '' : 'a'.repeat(64), sourceBackupReference: '', backupName: params.has('source') ? '' : 'verified-panel-backup.db', nodes: params.has('source') ? [] : nodes.map((node, i) => ({ nodeId: node.id, legacyNodeId: node.id, phase: params.has('review') && i === 0 ? 'review' : i === 0 ? 'native' : 'legacy', sourceDigest: 'a'.repeat(64), usageCutoff: i === 0 ? stamp : null, errorCode: params.has('review') && i === 0 ? 'XNODE_ROLLBACK_PENDING' : '' })) }
let latest: OperationReceipt | null = null
const pause = async () => { if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 2000)) }
async function check(name: string): Promise<void> {
  trace.calls.push(name); await pause()
  if (params.has('error') && trace.reloads++ === 0) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
}
function receipt(kind: string): OperationReceipt { latest = { id: crypto.randomUUID(), kind, status: 'queued', createdAt: stamp, updatedAt: stamp }; return latest }
xnodeApi.nodes = async () => { await check('nodes'); return { items: structuredClone(nodes) } }
xnodeApi.node = async id => ({ node: nodes.find(node => node.id === id)!, settings: { ddns: [], certificates: {}, inboundCertificates: {} } })
xnodeApi.configurations = async () => ({ items: structuredClone(configurations) })
xnodeApi.snippets = async () => ({ items: [] })
xnodeApi.groups = async () => ({ items: params.has('empty') ? [] : [{ id: 'group-one', name: 'Global group', enabled: true, bindings: nodes.map(node => ({ nodeId: node.id, inboundTag: 'paid' })) }] })
xnodeApi.endpoints = async () => ({ items: params.has('empty') ? [] : nodes.map(node => ({ id: node.id, nodeId: node.id, inboundTag: 'paid', name: node.name, address: `${node.countryCode?.toLowerCase()}.proxy.example`, port: 443, enabled: true, hidden: false, overrides: {} })) })
xnodeApi.metrics = async () => ({ nodes: params.has('empty') ? [] : nodes.map((node, i) => ({ nodeId: node.id, connected: i === 0, ready: i === 0, synchronized: i === 0, stale: i !== 0, version: 'v1.0.2', xrayVersion: '26.10.10', capabilities: ['traffic_periods_v1', 'traffic_period_barriers_v1'], reportedAt: stamp, machine: { cpu_usage: [12, 18], memory: { total: '8589934592', used: '2147483648' }, host: { hostname: node.name, platform: 'debian', platformVersion: '13' }, errors: {} }, certificates: {}, activeConnections: i === 0 ? 8 : 0, usersOnline: i === 0 ? 2 : 0, uploadBytesPerSecond: i === 0 ? 250000 : 0, downloadBytesPerSecond: i === 0 ? 1500000 : 0, logs: [] })), onlineNodes: nodes.length ? 1 : 0, totalNodes: nodes.length, activeConnections: nodes.length ? 8 : 0, uploadBytesPerSecond: 250000, downloadBytesPerSecond: 1500000, fetchedAt: stamp })
xnodeApi.global = async () => global
xnodeApi.saveGlobal = async value => { trace.calls.push('save-global'); global = value; return global }
xnodeApi.enroll = async id => { trace.calls.push(`enroll:${id}`); return { masterUrl: 'wss://panel.example/v1/agents/connect', token: 'fixture-token', protocol: 'x-node.v1' } }
xnodeApi.validate = async () => ({ valid: true, needsAgentValidation: false, errorCode: '' })
xnodeApi.migration = async () => { await check('migration'); return structuredClone(migration) }
xnodeApi.previewMigration = async () => { trace.calls.push('preview'); migration = { ...migration, phase: 'preview', snapshotDigest: 'b'.repeat(64), sourceBackupReference: 'verified-source-backup' }; return { digest: migration.snapshotDigest, nodes: nodes.length, groups: 1, accounts: 12, endpoints: nodes.length, issues: params.has('issue') ? [{ resourceId: 'legacy-ss', code: 'UNSUPPORTED_SHADOWSOCKS', blocking: true }] : [] } }
xnodeApi.importMigration = async () => { trace.calls.push('import'); migration = { ...migration, phase: 'active', backupName: 'verified-panel-backup.db', nodes: nodes.map(node => ({ nodeId: node.id, legacyNodeId: node.id, phase: 'legacy', sourceDigest: migration.snapshotDigest, usageCutoff: null, errorCode: '' })) }; return receipt('xnode_migration') }
xnodeApi.migrateNode = async (id, action) => { trace.calls.push(`${action}:${id}`); migration = { ...migration, nodes: migration.nodes.map(node => node.nodeId === id ? { ...node, phase: action === 'cutover' ? 'native' : 'legacy', errorCode: '' } : node) }; return receipt('xnode_migration') }
memberOperationsApi.getOperation = async () => ({ ...latest!, status: params.has('pending') ? 'pending_review' : 'succeeded', updatedAt: new Date().toISOString() })
Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { trace.copies.push(value) } } })
Object.assign(window, { __xnodeAudit: trace })
const app = createApp(XNodeAudit)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: XNodeAudit }] })).use(ui).mount('#app')
