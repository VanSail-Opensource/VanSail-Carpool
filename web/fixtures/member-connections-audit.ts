import { createApp, h } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import App from '@nuxt/ui/components/App.vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { setExperience } from '@/utils/experience'
import { setLocale } from '@/i18n'
import { memberConnectionsApi, type MemberConnection } from '@/api/memberConnections'
import ConnectionsPage from '@/components/connections/ConnectionsPage.vue'
import { ApiError } from '@/api/http'
import '@/styles/main.css'

const params = new URLSearchParams(location.search)
setExperience('miniapp'); setLocale(params.has('zh') ? 'zh-CN' : 'en')
const stamp = new Date().toISOString(), expiry = new Date(Date.now() + 86400000).toISOString()
const items: MemberConnection[] = params.has('empty') ? [] : Array.from({ length: 61 }, (_, i) => ({ ip: i ? `8.8.4.${i}` : '2001:4860::8888', initialLeaseAt: stamp, lastObservedAt: i === 1 ? null : stamp, expiresAt: expiry }))
const trace = { grants: [] as string[], loads: 0 }
Object.assign(window, { __connectionAudit: trace })
const pause = () => new Promise(resolve => setTimeout(resolve, params.has('slow') ? 1800 : 30))
memberConnectionsApi.list = async (search, page) => {
  trace.loads++; await pause()
  if (params.has('error')) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
  const filtered = items.filter(item => item.ip.includes(search)), selected = Math.min(page, Math.max(1, Math.ceil(filtered.length / 25)))
  return { items: structuredClone(filtered.slice((selected - 1) * 25, selected * 25)), page: selected, pageSize: 25, total: filtered.length, status: params.has('stale') ? 'stale' : params.has('unavailable') ? 'unavailable' : filtered.length ? 'available' : 'empty', incomplete: params.has('incomplete'), canGrant: !params.has('disabled'), settings: { enabled: !params.has('disabled'), durationHours: 24 }, fetchedAt: stamp }
}
memberConnectionsApi.grant = async ip => {
  trace.grants.push(ip); await pause()
  if (params.has('grant-error') || !ip.includes(':') && !/^\d+\.\d+\.\d+\.\d+$/.test(ip)) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' })
  const existing = items.find(item => item.ip === ip)
  if (existing) return existing
  const added: MemberConnection = { ip, initialLeaseAt: new Date().toISOString(), lastObservedAt: null, expiresAt: expiry }
  items.push(added); return added
}
createApp({ render: () => h(App, { locale: params.has('zh') ? zh_cn : en }, () => h(ConnectionsPage)) }).use(ui).mount('#app')
