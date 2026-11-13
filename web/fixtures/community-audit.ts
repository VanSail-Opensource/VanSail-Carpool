import '@/styles/main.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { ipLookupApi } from '@/api/ipLookup'
import type { CommunityMembership } from '@/api/types'
import { t, setLocale } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { setExperience } from '@/utils/experience'
import CommunityAudit from './CommunityAudit.vue'

const parameters = new URLSearchParams(location.search)
setExperience('browser')
setLocale(parameters.has('zh') ? 'zh-CN' : 'en')
const now = new Date().toISOString()
const membership: CommunityMembership = { activeCombo: !parameters.has('locked'), groupJoined: parameters.has('joined'), channelJoined: parameters.has('joined'), user: { id: 'audit-user', telegramId: '42', firstName: 'Mira', lastName: '', telegramUsername: 'mira', displayName: '@mira', username: 'mira', role: 'user', onboardingState: 'complete', policyAcceptedAt: now, agreementRevision: 1, groupJoined: false, channelJoined: false, displayCurrency: 'TXB', recoveryReason: '', createdAt: now, updatedAt: now } }
const state = { reads: 0, invites: 0, membershipFinished: 0, inviteFinished: 0, accessReads: 0, accessFinished: 0, deferAccess: parameters.has('defer-access'), opened: [] as string[] }
let releaseMembership: (() => void) | undefined, releaseInvite: (() => void) | undefined
api.checkCommunityMembership = async () => {
  state.reads++
  if (parameters.has('slow')) await new Promise(resolve => setTimeout(resolve, 1800))
  if (parameters.has('defer-membership')) await new Promise<void>(resolve => { releaseMembership = resolve })
  if (parameters.has('error')) throw new Error('Constructed community failure')
  state.membershipFinished++
  return membership
}
api.createCommunityInvite = async () => {
  state.invites++
  if (parameters.has('defer-invite')) await new Promise<void>(resolve => { releaseInvite = resolve })
  state.inviteFinished++
  return { url: 'https://t.me/constructed-audit', expiresAt: now }
}
window.open = (url?: string | URL) => { state.opened.push(String(url)); return null }
const pinia = createPinia()
const session = useSessionStore(pinia)
session.updateSession({ authenticated: true, user: membership.user })
const pendingAccess: Array<{ owner: string; release: (activeCombo: boolean, minor: string) => void }> = []
function readAccess(): Promise<{ activeCombo: boolean; balance: { minor: string; currency: 'TXB'; display: string } }> {
  const owner = session.user?.id ?? ''
  state.accessReads++
  const result = (activeCombo: boolean, minor: string) => {
    state.accessFinished++
    return { activeCombo, balance: { minor, currency: 'TXB' as const, display: `${(Number(minor) / 100).toFixed(2)} TXB` } }
  }
  if (!state.deferAccess) return Promise.resolve(result(true, '100'))
  return new Promise(resolve => { pendingAccess.push({ owner, release: (combo, minor) => resolve(result(combo, minor)) }) })
}
api.getCommunityAccess = async () => ({ activeCombo: (await readAccess()).activeCombo })
api.getBalance = async () => ({ balance: (await readAccess()).balance, paymentMethods: [], pendingPaymentOrder: null,
  addAmountLimits: { minimum: { minor: '100', currency: 'TXB', display: '1.00 TXB' }, maximum: { minor: '10000', currency: 'TXB', display: '100.00 TXB' }, updatedAt: now } })
ipLookupApi.state = async () => ({ enabled: false, lookupFee: null, refreshFee: null, allowance: null })
Object.assign(window, { __communityAuditLocale: setLocale, __communityAudit: {
  state, clearOwner: () => session.clear(), user: () => session.user?.id ?? null,
  releaseMembership: () => releaseMembership?.(), releaseInvite: () => releaseInvite?.(),
  switchOwner: () => session.updateSession({ authenticated: true, user: { ...membership.user, id: 'audit-user-2' } }),
  releaseAccess: (owner: string, combo: boolean, minor: string) => {
    for (const pending of pendingAccess.filter(item => item.owner === owner)) pending.release(combo, minor)
  },
} })
const app = createApp(CommunityAudit)
app.config.globalProperties.$t = t
app.use(pinia)
app.use(createRouter({ history: createWebHistory(), routes: [{ path: '/fixtures/community-audit.html', component: CommunityAudit }] }))
app.use(ui)
app.mount('#app')
