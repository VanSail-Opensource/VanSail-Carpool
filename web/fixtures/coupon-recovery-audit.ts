import { createApp, h, shallowRef } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { featuresApi, type CouponDefinition } from '@/api/features'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import CouponRecoveryAudit from './CouponRecoveryAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser')
initializeBrowserTheme()
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const visible = shallowRef(true)
let coupon: CouponDefinition = {
  id: 'coupon-audit', code: 'RETAINED', name: 'Reviewed coupon', kind: 'purchase_recurring', discountMode: 'percent',
  valueMinorOrBps: '1250', percentCapMinor: null, eligibleComboIds: ['retained-plan'], eligibleSquadIds: [],
  expiresAt: '2026-12-01T08:15:30.123Z', globalUseLimit: null, perUserUseLimit: 1, active: true, usageCount: 2,
  createdAt: '2026-11-13T00:00:00Z', updatedAt: '2026-11-13T00:00:00Z',
}
const state = { reads: 0, optionReads: 0, writes: [] as { id: string | null; body: Partial<CouponDefinition> }[], deactivations: [] as string[], applied: 0, pending: 0, recover: false, optionsRecovered: false }
const pending: Array<{ resolve: () => void; reject: (error: Error) => void }> = []
async function hold(): Promise<void> {
  if (!params.has('defer')) return
  state.pending++
  await new Promise<void>((resolve, reject) => { pending.push({ resolve, reject }) })
}
featuresApi.getAdminCoupons = async () => {
  state.reads++
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 900))
  if (params.has('refresh-error') && state.applied > 0 && !state.recover) throw new Error('Constructed coupon read failure')
  return { items: [structuredClone(coupon)] }
}
api.getAdminResource = async resource => {
  if (resource !== 'combos' && resource !== 'squad-products') throw new Error('Unexpected constructed coupon options')
  state.optionReads++
  if (params.has('options-error') && !state.optionsRecovered) throw new Error('Constructed coupon options failure')
  return { items: [] } as never
}
featuresApi.saveAdminCoupon = async (id, body) => {
  state.writes.push({ id, body: structuredClone(body) })
  await hold()
  state.applied++
  coupon = { ...coupon, ...body, id: id ?? 'created-coupon', updatedAt: '2026-11-13T01:00:00Z' }
  return structuredClone(coupon)
}
featuresApi.deactivateAdminCoupon = async id => {
  state.deactivations.push(id)
  await hold()
  state.applied++
  coupon = { ...coupon, active: false }
}
Object.assign(window, { __couponRecoveryAudit: Object.assign(state, {
  retire: () => { visible.value = false },
  release: async (fail = false) => {
    for (const item of pending.splice(0)) { if (fail) item.reject(new Error('Constructed coupon refusal')); else item.resolve() }
    state.pending = 0
    await new Promise(resolve => setTimeout(resolve, 0))
  },
}) })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: CouponRecoveryAudit }] })
await router.push('/')
const app = createApp({ render: () => h(CouponRecoveryAudit, { visible: visible.value }) }).use(createPinia()).use(router).use(ui)
app.config.globalProperties.$t = t
app.mount('#app')
