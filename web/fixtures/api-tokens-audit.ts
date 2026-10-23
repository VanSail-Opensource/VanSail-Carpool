import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { adminAPITokensApi, type PanelAPIToken } from '@/api/adminAPITokens'
import { ApiError } from '@/api/http'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import APITokensAudit from './APITokensAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser'); initializeBrowserTheme(); setLocale(params.has('zh') ? 'zh-CN' : 'en')
const trace = { calls: [] as string[], copied: [] as string[], failures: 0 }
const stamp = new Date().toISOString()
let items: PanelAPIToken[] = params.has('empty') ? [] : [
  { id: 'migration', name: 'Migration assistant', prefix: 'vsc_example1', scopes: ['xnode:write', 'xnode:write'], createdAt: stamp, expiresAt: '2099-01-01T00:00:00Z', lastUsedAt: stamp, revokedAt: null },
  { id: 'expired', name: 'Expired monitoring token', prefix: 'vsc_example2', scopes: ['xnode:read'], createdAt: '2000-01-01T00:00:00Z', expiresAt: '2000-02-01T00:00:00Z', lastUsedAt: null, revokedAt: null },
  { id: 'revoked', name: 'Old integration', prefix: 'vsc_example3', scopes: ['admin:read'], createdAt: stamp, expiresAt: null, lastUsedAt: null, revokedAt: stamp },
]
async function pause(): Promise<void> { if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 1800)) }
adminAPITokensApi.list = async () => { trace.calls.push('list'); await pause(); if (params.has('error') && trace.failures++ === 0) throw new ApiError(503, { code: 'API_TOKEN_UNAVAILABLE', message: '' }); return { items: structuredClone(items) } }
adminAPITokensApi.create = async body => {
  trace.calls.push('create'); await pause()
  if (params.has('create-error')) throw new ApiError(409, { code: 'API_TOKEN_LIMIT', message: '' })
  const token = 'vsc_' + 'A'.repeat(43)
  const item: PanelAPIToken = { id: crypto.randomUUID(), name: body.name, prefix: token.slice(0, 12), scopes: body.scopes, createdAt: stamp, expiresAt: body.expiresInDays === 0 ? null : new Date(Date.now() + (body.expiresInDays ?? 30) * 86400000).toISOString(), lastUsedAt: null, revokedAt: null }
  items = [item, ...items]; return { item, token }
}
adminAPITokensApi.revoke = async id => { trace.calls.push(`revoke:${id}`); await pause(); if (params.has('revoke-error')) throw new ApiError(503, { code: 'API_TOKEN_UNAVAILABLE', message: '' }); items = items.map(item => item.id === id ? { ...item, revokedAt: stamp } : item) }
Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { if (params.has('copy-error')) throw new Error('Fixture copy failure'); trace.copied.push(value) } } })
if (params.has('copy-error')) document.execCommand = () => false
Object.assign(window, { __apiTokensAudit: trace })
const app = createApp(APITokensAudit)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: APITokensAudit }] })).use(ui).mount('#app')
