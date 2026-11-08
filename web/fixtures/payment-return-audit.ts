import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { browserAccessApi } from '@/api/browserAccess'
import { ApiError } from '@/api/http'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import PaymentReturnAudit from './PaymentReturnAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser')
initializeBrowserTheme()
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const state = { reads: 0, capabilities: [] as string[], configReads: 0 }
api.getPaymentReturnStatus = async (provider, id, capability) => {
  state.reads++
  state.capabilities.push(capability)
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 3500))
  if (params.has('denied') || (params.has('expires') && state.reads > 1)) throw new ApiError(401, { code: 'REQUEST_SIGNATURE_REQUIRED', message: '' })
  if (params.has('missing')) throw new ApiError(404, { code: 'PAYMENT_NOT_FOUND', message: '' })
  if (params.has('error')) throw new ApiError(503, { code: 'UNAVAILABLE', message: '' })
  return {
    id, provider, providerRail: 'alipay', status: params.has('pending') || params.has('expires') ? 'pending' : 'paid',
    txb: { currency: 'TXB', minor: '2000', display: '20.00 TXB' }, payableAmount: '20.00', payableCurrency: 'CNY',
    actualCryptoAmount: null, actualCryptoCurrency: null, createdAt: '2026-11-08T00:00:00Z', paidAt: params.has('pending') ? null : '2026-11-08T00:01:00Z',
  }
}
browserAccessApi.config = async () => { state.configReads++; return { miniAppUrl: 'https://t.me/fixture_bot/app' } }
Object.assign(window, { __paymentReturnAudit: state })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: PaymentReturnAudit }] })
await router.push({ path: '/', query: { paymentOrder: params.has('empty') ? '' : 'checkout-order', provider: 'ezpay', paymentCapability: params.has('legacy') ? '' : 'v2.checkout-fixture' } })
const app = createApp(PaymentReturnAudit).use(createPinia()).use(router).use(ui)
app.config.globalProperties.$t = t
app.mount('#app')
