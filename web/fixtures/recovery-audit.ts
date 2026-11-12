import '@/styles/main.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import App from '@/App.vue'
import { api } from '@/api/client'
import { displayCurrencyApi } from '@/api/displayCurrency'
import { memberOperationsApi } from '@/api/memberOperations'
import type { FeaturePaymentMethod, FeaturePaymentOrder } from '@/api/features'
import type { DisplayCurrencyPreference, OperationReceipt, Session } from '@/api/types'
import { useDisplayCurrency } from '@/composables/useDisplayCurrency'
import { useSessionStore } from '@/stores/session'
import { setExperience } from '@/utils/experience'
import { setLocale, t } from '@/i18n'
import RecoveryAudit from './RecoveryAudit.vue'

// All operations below are constructed and have no backend/provider effects.
setExperience('browser')
const params = new URLSearchParams(location.search)
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const now = new Date().toISOString()
const preference = (currency: DisplayCurrencyPreference['currency']): DisplayCurrencyPreference => ({
  currency, rates: { cnyTxbPerUnit: '10', usdTxbPerUnit: '70' },
})
const pendingCurrency: Array<(value: DisplayCurrencyPreference) => void> = []
const state = { reads: 0, saves: 0, cancels: 0, orderReads: 0, recover: false }
displayCurrencyApi.get = async () => {
  state.reads++
  if (params.has('currency-slow')) return new Promise(resolve => { pendingCurrency.push(resolve) })
  return preference('TXB')
}
displayCurrencyApi.update = async currency => {
  state.saves++
  if (params.has('currency-error') && !state.recover) throw new Error('Constructed currency save refusal')
  return preference(currency)
}

const methods: FeaturePaymentMethod[] = params.has('empty') ? [] : [{
  id: 'ezpay:audit:alipay', provider: 'ezpay', profileId: 'audit', providerName: 'Audit provider',
  rail: 'alipay', name: 'Alipay', currency: 'CNY', cryptoCurrency: '', network: '', networkName: '',
  available: true, note: '', mode: 'order',
}]
let order: FeaturePaymentOrder = {
  id: 'audit-order', methodId: 'ezpay:audit:alipay', provider: 'ezpay', providerRail: 'alipay', status: 'pending',
  txb: { currency: 'TXB', minor: '2000', display: '20.00 TXB' }, payableAmount: '2.00', payableCurrency: 'CNY',
  rateSnapshot: '10', rateDirection: 'txb_per_currency', paymentUrl: 'https://pay.example/audit-order',
  qrPayload: 'https://pay.example/audit-order', receivingAddress: null, actualCryptoAmount: null,
  actualCryptoCurrency: null, expiresAt: new Date(Date.now() + 3600000).toISOString(), paidAt: null,
  refundedAt: null, cancelledAt: null, cancelReason: '', providerCancelStatus: '', createdAt: now, updatedAt: now,
}
const receipt = (status: OperationReceipt['status']): OperationReceipt => ({
  id: 'audit-cancel', kind: 'payment_cancel', status, createdAt: now, updatedAt: now,
})
api.getPaymentOrder = async () => { state.orderReads++; return { ...order } }
api.cancelPaymentOrder = async () => { state.cancels++; return { paymentOrderId: order.id, operation: receipt('queued') } }
memberOperationsApi.getOperation = async () => receipt(params.has('cancel-succeeded') ? 'succeeded' : 'failed')

const router = createRouter({ history: createMemoryHistory(), routes: [{
  path: '/', component: RecoveryAudit, meta: { immersive: true },
  props: { methods, pendingOrder: params.has('empty') ? null : { ...order } },
}] })
const app = createApp(App).use(createPinia()).use(router).use(ui)
app.config.globalProperties.$t = t
const session = useSessionStore()
const currency = useDisplayCurrency()
function owner(id: string, displayCurrency: DisplayCurrencyPreference['currency']): Session {
  return { authenticated: true, user: {
    id, telegramId: '42', firstName: 'Audit', lastName: '', telegramUsername: '', displayName: 'Audit',
    username: null, role: 'user', onboardingState: 'intro', groupJoined: false, channelJoined: false,
    policyAcceptedAt: null, agreementRevision: 0, recoveryReason: '', displayCurrency, createdAt: now, updatedAt: now,
  } }
}
session.updateSession(owner('audit-owner-1', 'TXB'))
Object.assign(window, { __recoveryAudit: {
  state,
  releaseCurrency: (value: DisplayCurrencyPreference['currency']) => pendingCurrency.shift()?.(preference(value)),
  switchOwner: () => session.updateSession(owner('audit-owner-2', 'USD')),
  pay: () => { order = { ...order, status: 'paid', paidAt: now } },
  snapshot: () => ({ currency: currency.currency.value, rates: currency.rates.value,
    loading: currency.loading.value, saving: currency.saving.value, user: session.user?.id ?? null, ...state }),
} })
await router.isReady()
app.mount('#app')
