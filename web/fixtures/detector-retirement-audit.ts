import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import DetectorRetirementAudit from './DetectorRetirementAudit.vue'
import RetiredControlsAudit from './RetiredControlsAudit.vue'
import MemberNodesAudit from './MemberNodesAudit.vue'
import { xnodeMemberApi, type RoutingView } from '@/api/xnodeMember'
import { preferencesApi } from '@/api/preferences'
import { usePreferencesStore } from '@/stores/preferences'

const params = new URLSearchParams(location.search)
setExperience(params.has('member') ? 'miniapp' : 'browser')
if (params.has('member')) await import('@/styles/main.css')
else { await import('@/browser/styles.css'); initializeBrowserTheme() }
setLocale(params.has('zh') ? 'zh-CN' : 'en')
if (params.has('native')) {
  const trace = { saves: [] as unknown[], failMetrics: params.has('metrics-error'), failFormats: params.has('format-error') }
  const custom: RoutingView = {
    plan: { revision: 1, phase: 'active', choices: [{ id: 'constructed-route', inboundNodeId: 'tokyo', outboundNodeId: 'singapore', inboundEndpointId: 'tokyo-host', outboundEndpointId: 'singapore-host', templateId: 'work' }] },
    nodes: [{ id: 'tokyo', name: 'Tokyo transit' }, { id: 'singapore', name: 'Singapore transit' }],
    endpoints: [{ id: 'tokyo-host', nodeId: 'tokyo', name: 'Tokyo entry', address: 'tokyo.example', port: 443 }, { id: 'singapore-host', nodeId: 'singapore', name: 'Singapore exit', address: 'singapore.example', port: 443 }],
    templates: [{ id: 'work', name: 'Work services', match: { domain: ['geosite:google'] } }], paths: [],
  }
  const pause = () => new Promise(resolve => globalThis.setTimeout(resolve, params.has('slow') ? Number(params.get('slow')) || 1500 : 0))
  let failed = false
  xnodeMemberApi.metrics = async () => { await pause(); if (trace.failMetrics) throw new Error('fixture metrics'); return { activityStatus: 'online', activityExpiresAt: new Date(Date.now() + 15000).toISOString(), connections: 3, uploadBytesPerSecond: 1048576, downloadBytesPerSecond: 2097152, limited: true, incomplete: true, fetchedAt: new Date().toISOString(), reasons: [{ nodeId: 'tokyo', reason: 'global_connection_limit', scope: 'new_connection' }] } }
  xnodeMemberApi.subscription = async () => { await pause(); if (trace.failFormats) throw new Error('fixture formats'); return { username: 'member', url: `${location.origin}/subscription/${"a".repeat(43)}`, downloadUrl: `${location.origin}/api/v1/subscriptions/${"a".repeat(43)}`, usedBytes: '9007199254740993', limitBytes: '9999999999999999', expiresAt: null, active: !params.has('empty'), ready: true, firstSyncedAt: '2026-11-13T00:00:00Z', incomplete: false, formats: { base64: !params.has('empty') && !params.has('formats-none'), stash: !params.has('empty') && !params.has('formats-none'), native: true, xray: true, 'sing-box': true, clash: false } } }
  xnodeMemberApi.routing = async () => {
    await pause()
    if (params.has('error') && !failed) { failed = true; throw new Error('Constructed routing read failure') }
    return params.has('empty') ? { ...structuredClone(custom), nodes: [], endpoints: [], templates: [], paths: [], plan: { ...custom.plan, choices: [] } } : structuredClone(custom)
  }
  xnodeMemberApi.previewRouting = async body => {
    await pause()
    const paths = body.choices.map(choice => ({ nodes: [choice.inboundNodeId, choice.outboundNodeId], rules: [choice.id], conditions: [], exclusions: [], excludedRules: [] }))
    return { paths, trace: null }
  }
  xnodeMemberApi.saveRouting = async (body, key) => {
    await pause()
    trace.saves.push({ body: structuredClone(body), key })
    custom.plan = { revision: body.revision + 1, choices: structuredClone(body.choices), phase: 'active', operationId: key }
    const stamp = new Date().toISOString()
    return { id: key, kind: 'xnode_routing', status: 'succeeded', createdAt: stamp, updatedAt: stamp, completedAt: stamp }
  }
  Object.assign(window, { memberNodesAudit: trace })
}
const app = createApp(params.has('native') ? MemberNodesAudit : params.has('legacy') ? RetiredControlsAudit : DetectorRetirementAudit)
app.config.globalProperties.$t = t
const pinia = createPinia()
preferencesApi.get = async () => ({ paygEnabled: true, paygSquads: {}, activeCombo: true, showAroundTx: true, showActivity: false, includeNodePrices: true, showReferralUsername: true, notifications: { combos: true, traffic: true, money: true, activity: true, account: true } })
if (params.has('member')) usePreferencesStore(pinia).bind('detector-audit-member')
app.use(pinia).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: DetectorRetirementAudit, meta: { adminSection: 'users' } }] })).use(ui).mount('#app')
