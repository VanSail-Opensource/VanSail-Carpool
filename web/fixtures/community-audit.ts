import '@/styles/main.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { t, setLocale } from '@/i18n'
import CommunityAudit from './CommunityAudit.vue'

const parameters = new URLSearchParams(location.search)
const now = new Date().toISOString()
api.checkCommunityMembership = async () => {
  if (parameters.has('slow')) await new Promise(resolve => setTimeout(resolve, 10000))
  if (parameters.has('error')) throw new Error('Constructed community failure')
  return { activeCombo: !parameters.has('locked'), groupJoined: parameters.has('joined'), channelJoined: parameters.has('joined'), user: { id: 'audit-user', telegramId: '42', firstName: 'Mira', lastName: '', telegramUsername: 'mira', displayName: '@mira', username: 'mira', role: 'user', onboardingState: 'complete', policyAcceptedAt: now, agreementRevision: 1, groupJoined: false, channelJoined: false, displayCurrency: 'TXB', recoveryReason: '', createdAt: now, updatedAt: now } } as never
}
api.createCommunityInvite = async () => ({ url: 'https://t.me/example', expiresAt: now }) as never
Object.assign(window, { __communityAuditLocale: setLocale })
const app = createApp(CommunityAudit)
app.config.globalProperties.$t = t
app.use(createPinia())
app.use(createRouter({ history: createWebHistory(), routes: [{ path: '/fixtures/community-audit.html', component: CommunityAudit }] }))
app.use(ui)
app.mount('#app')
