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
import { xnodeMemberApi, type XNodeCustomization } from '@/api/xnodeMember'

const params = new URLSearchParams(location.search)
setExperience(params.has('member') ? 'miniapp' : 'browser')
if (params.has('member')) await import('@/styles/main.css')
else { await import('@/browser/styles.css'); initializeBrowserTheme() }
setLocale(params.has('zh') ? 'zh-CN' : 'en')
if (params.has('native')) {
  const trace = { saves: [] as unknown[], failMetrics: params.has('metrics-error'), failFormats: params.has('format-error') }
  const custom: XNodeCustomization = { nodeId: 'tokyo', revision: 1, outbounds: [], routing: [] }
  const pause = () => new Promise(resolve => globalThis.setTimeout(resolve, params.has('slow') ? Number(params.get('slow')) || 1500 : 0))
  let failed = false
  xnodeMemberApi.metrics = async () => { await pause(); if (trace.failMetrics) throw new Error('fixture metrics'); return { connections: 3, uploadBytesPerSecond: 1048576, downloadBytesPerSecond: 2097152, limited: true, incomplete: true, fetchedAt: new Date().toISOString(), reasons: [{ nodeId: 'tokyo', reason: 'global_connection_limit', scope: 'new_connection' }] } }
  xnodeMemberApi.subscription = async () => { await pause(); if (trace.failFormats) throw new Error('fixture formats'); return { username: 'member', url: `${location.origin}/subscription/${"a".repeat(43)}`, downloadUrl: `${location.origin}/api/v1/subscriptions/${"a".repeat(43)}`, usedBytes: '9007199254740993', limitBytes: '9999999999999999', expiresAt: null, active: !params.has('empty'), incomplete: false, formats: { base64: !params.has('empty') && !params.has('formats-none'), stash: !params.has('empty') && !params.has('formats-none'), native: true, xray: true, 'sing-box': true, clash: false } } }
  xnodeMemberApi.templates = async () => { await pause(); if (params.has('error') && !failed) { failed = true; throw new Error('fixture') }; return { items: params.has('empty') ? [] : [{ nodeId: 'tokyo', name: 'Tokyo transit', outbound: { tag: 'peer-tokyo', protocol: 'trojan', settings: { servers: [{ address: '1.1.1.1', port: 443, password: 'fixture' }] } } }] } }
  xnodeMemberApi.routing = async () => { await pause(); return structuredClone(custom) }
  xnodeMemberApi.saveRouting = async (_id, kind, body) => { await pause(); trace.saves.push({ kind, ...body }); custom.revision++; if (kind === 'outbounds') custom.outbounds = body.objects; else custom.routing = body.objects; return structuredClone(custom) }
  Object.assign(window, { memberNodesAudit: trace })
}
const app = createApp(params.has('native') ? MemberNodesAudit : params.has('legacy') ? RetiredControlsAudit : DetectorRetirementAudit)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: DetectorRetirementAudit, meta: { adminSection: 'users' } }] })).use(ui).mount('#app')
