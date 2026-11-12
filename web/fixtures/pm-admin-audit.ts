import '@/browser/styles.css'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { pmApi } from '@/api/pm'
import { operationReceiptLoaderKey, type OperationReceiptLoader } from '@/composables/operationReceiptLoader'
import { ApiError } from '@/api/http'
import type { OperationReceipt, PMConversation, PMDelivery } from '@/api/types'
import { setLocale, t } from '@/i18n'
import PMAdminAudit from './PMAdminAudit.vue'

setExperience('browser')
initializeBrowserTheme()
const params = new URLSearchParams(location.search)
setLocale(params.has('zh') ? 'zh-CN' : 'en')
if (params.has('reduced')) {
  const native = window.matchMedia.bind(window)
  window.matchMedia = query => { const media = native(query); if (query === '(prefers-reduced-motion: reduce)') Object.defineProperty(media, 'matches', { value: true }); return media }
}
const date = new Date().toISOString()
const make = (id: string, name: string): PMConversation => ({ id, userId: id, telegramId: '42', name, username: 'member_' + id, chatId: '-100123', topicId: '50', profileMessageId: '51', topicState: 'ready', profileState: 'ready', blocked: false, muted: false, createdAt: date, updatedAt: date })
let conversations: PMConversation[] = [make('ada', 'Ada'), { ...make('mira', 'Mira'), muted: true }, { ...make('review', 'Member with a long Telegram display name'), topicId: null, profileMessageId: null, topicState: 'pending_review', profileState: 'missing' }]
if (params.has('empty')) conversations = []
const state = { changes: 0, repairs: 0, reads: 0, polls: 0, lastRepair: null as unknown, failure: params.has('error') }
const receipts = new Map<string, OperationReceipt>()
async function loadConversation(): Promise<PMConversation | null> {
  state.reads++
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 10_000))
  if (state.failure) throw new ApiError(503, { code: 'PM_QUEUE_UNAVAILABLE', message: '' })
  return conversations.find(item => item.id === params.get('user')) ?? conversations[0] ?? null
}
const deliveries: PMDelivery[] = [
  { operationId: 'uncertain-delivery', status: 'pending_review', direction: 'inbound', sourceChatId: '42', sourceMessageId: '10', resultMessageId: null, topicId: '50', errorCode: 'PM_DELIVERY_UNCERTAIN', createdAt: date },
  { operationId: 'sent-delivery', status: 'succeeded', direction: 'outbound', sourceChatId: '-100123', sourceMessageId: '52', resultMessageId: '20', topicId: '50', errorCode: '', createdAt: date },
]
pmApi.deliveries = async () => {
  if (params.has('reviewslow')) await new Promise(resolve => setTimeout(resolve, 10_000))
  if (params.has('reviewerror')) throw new ApiError(503, { code: 'PM_QUEUE_UNAVAILABLE', message: '' })
  return { items: params.has('reviewempty') ? [] : deliveries }
}
function receipt(): OperationReceipt { const item: OperationReceipt = { id: 'pm-command-' + (state.changes + state.repairs), kind: 'telegram_pm_profile', status: 'queued', errorCode: null, createdAt: date, updatedAt: date, completedAt: null }; receipts.set(item.id, item); return item }
pmApi.moderate = async (id, body) => {
  if (params.has('moderationerror')) throw new ApiError(422, { code: 'INVALID_PM_UPDATE', message: '' })
  state.changes++
  conversations = conversations.map(item => item.id === id ? { ...item, ...body } : item)
  return receipt()
}
pmApi.repair = async (id, body) => {
  state.repairs++; state.lastRepair = body
  conversations = conversations.map(item => item.id === id ? { ...item, topicId: body.topicId, profileMessageId: body.profileMessageId || '91', topicState: 'ready', profileState: 'ready' } : item)
  return receipt()
}
const loadOperation: OperationReceiptLoader = async (id) => {
  state.polls++
  if (params.has('pollerror')) throw new ApiError(503, { code: 'PM_QUEUE_UNAVAILABLE', message: '' })
  const item = receipts.get(id)!
  const next = { ...item, status: params.has('pending') ? 'processing' as const : 'succeeded' as const, completedAt: params.has('pending') ? null : date }
  receipts.set(id, next); return next
}
Object.assign(window, { __pmAudit: state, __pmConversations: () => conversations, __pmLocale: setLocale })
const app = createApp(PMAdminAudit, { loadConversation }).provide(operationReceiptLoaderKey, loadOperation)
app.config.globalProperties.$t = t
app.use(createPinia())
const router = createRouter({ history: createWebHistory(), routes: [{ path: '/fixtures/pm-admin-audit.html', component: PMAdminAudit }] })
app.use(router); app.use(ui)
void router.isReady().then(() => app.mount('#app'))
