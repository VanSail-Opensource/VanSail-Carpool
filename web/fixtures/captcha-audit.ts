import '@/styles/main.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { ApiError } from '@/api/http'
import type { Session } from '@/api/types'
import { t, setLocale } from '@/i18n'
import type { Turnstile } from '@/utils/turnstile'
import { featuresApi } from '@/api/features'
import { adminBillingApi } from '@/api/adminBilling'
import CaptchaAudit from './CaptchaAudit.vue'

const params = new URLSearchParams(location.search)
const state = { calls: 0, resets: 0, verified: false, failure: params.has('rejected') || params.has('unavailable') }
const user: Session['user'] = { id: 'captcha-member', telegramId: '42', firstName: 'Mira', lastName: '', telegramUsername: 'mira', displayName: '@mira', username: null, role: 'user', onboardingState: 'intro', groupJoined: false, channelJoined: false, policyAcceptedAt: null, agreementRevision: 0, recoveryReason: '', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }
api.getMe = async () => ({ authenticated: true, user })
api.verifyCaptcha = async () => {
  state.calls++
  await new Promise(resolve => setTimeout(resolve, Number(params.get('delay') ?? 700)))
  if (state.failure) throw new ApiError(params.has('unavailable') ? 503 : 422, { code: params.has('unavailable') ? 'CAPTCHA_UNAVAILABLE' : 'CAPTCHA_REJECTED', message: '' })
  state.verified = true
  return { authenticated: true, user, captcha: { required: false, siteKey: '', action: 'txc_first_entry' } }
}
if (params.has('mock')) {
  let options: Parameters<Turnstile['render']>[1] | null = null
  let host: HTMLElement | null = null
  window.turnstile = {
    render(element, next) {
      if (params.has('loaderror')) throw new Error()
      options = next; host = element
      const button = document.createElement('button')
      button.textContent = t('captcha.title')
      button.style.cssText = 'width:100%;min-height:65px;color:white;background:#191919;border:1px solid #444;border-radius:8px'
      button.onclick = () => options?.callback('constructed-token')
      host.append(button)
      return 'audit-widget'
    },
    remove: () => { host?.replaceChildren() },
    reset: () => { state.resets++ },
  }
  Object.assign(window, { __captchaExpire: () => options?.['expired-callback'](), __captchaProviderError: () => options?.['error-callback']() })
}
Object.assign(window, { __captchaAudit: state, __captchaLocale: setLocale })
if (params.has('admin')) {
  const settings = ['enabled', 'site_key', 'secret_key'].map(suffix => ({ key: `captcha.turnstile.${suffix}`, value: suffix === 'enabled' ? 'false' : '', encrypted: suffix === 'secret_key', configured: false, category: 'captcha', updatedAt: new Date().toISOString() }))
  settings.push(...['enabled', 'group_chat_id'].map(suffix => ({ key: `telegram.pm.${suffix}`, value: suffix === 'enabled' ? 'false' : '', encrypted: false, configured: false, category: 'telegram', updatedAt: new Date().toISOString() })))
  const writes: string[] = []
  api.getAdminResource = async () => {
    if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 10_000))
    if (params.has('error')) throw new ApiError(503, { code: 'CAPTCHA_UNAVAILABLE', message: '' })
    return { items: settings } as never
  }
  api.updateAdminSetting = async (key, value) => {
    const item = settings.find(setting => setting.key === key)!
    if (key === 'captcha.turnstile.enabled' && value === 'true' && !settings.filter(s => s.key.startsWith('captcha.') && !s.key.endsWith('.enabled')).every(s => s.configured)) throw new ApiError(422, { code: 'INVALID_SETTING', message: '' })
    if (key === 'telegram.pm.enabled' && value === 'true' && !settings.find(s => s.key === 'telegram.pm.group_chat_id')?.configured) throw new ApiError(422, { code: 'INVALID_SETTING', message: '' })
    writes.push(key); item.configured = true
    if (!item.encrypted) item.value = value
    return undefined as never
  }
  const money = (minor: string) => ({ currency: 'TXB' as const, minor, display: '' })
  const limits = { minimum: money('100'), maximum: money('10000'), updatedAt: new Date().toISOString() }
  api.getBalance = async () => ({ balance: money('0'), paymentMethods: [], addAmountLimits: limits, pendingPaymentOrder: null })
  adminBillingApi.updateAmountLimits = async () => limits
  api.getAdminPaymentProfiles = async () => ({ items: [] })
  const activity = { timezone: 'Asia/Shanghai', dailyRewardMinTxb: '1.25', dailyRewardMaxTxb: '2.25', groupMessageThreshold: 5, groupMessageRewardTxb: '1.25', updatedAt: null }
  featuresApi.getAdminActivitySettings = async () => activity as never
  featuresApi.saveAdminActivitySettings = async () => activity as never
  Object.assign(window, { __captchaSettingsAudit: { settings, writes } })
}
const app = createApp(CaptchaAudit)
app.config.globalProperties.$t = t
app.use(ui)
app.use(createPinia())
const router = createRouter({ history: createWebHistory(), routes: [{ path: '/fixtures/captcha-audit.html', component: CaptchaAudit }] })
app.use(router)
void router.isReady().then(() => app.mount('#app'))
