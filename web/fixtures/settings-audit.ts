import '@/styles/main.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { comboControlsApi } from '@/api/comboControls'
import { api } from '@/api/client'
import { preferencesApi, type GroupMemberTag, type UserPreferences } from '@/api/preferences'
import { memberOperationsApi } from '@/api/memberOperations'
import { ApiError } from '@/api/http'
import type { ComboControls, EarlyActivationQuote, OperationReceipt, Purchase } from '@/api/types'
import { usePreferencesStore } from '@/stores/preferences'
import { setLocale, t } from '@/i18n'
import SettingsAudit from './SettingsAudit.vue'

const params = new URLSearchParams(location.search)
if (params.has('reduced')) {
  const nativeMatchMedia = window.matchMedia.bind(window)
  window.matchMedia = query => {
    const media = nativeMatchMedia(query)
    if (query === '(prefers-reduced-motion: reduce)') Object.defineProperty(media, 'matches', { value: true })
    return media
  }
}
const now = Date.now()
const at = (offset: number) => new Date(now + offset).toISOString()
const day = 86400000
const squads = [{ uuid: 'squad-a', name: 'Tokyo', enabled: true, canDisable: true }, { uuid: 'squad-b', name: 'Amsterdam', enabled: true, canDisable: true }]
if (params.has('single')) { squads.splice(1); squads[0]!.canDisable = false }
function purchase(id: string, from: string, until: string): Purchase {
  const money = { currency: 'TXB' as const, minor: '1000', display: '10.00 TXB' }
  return { id, comboId: id, comboName: id === 'current' ? 'Standard' : 'Weekend', price: money, grossPrice: money,
    couponDiscount: { currency: 'TXB', minor: '0', display: '0.00 TXB' }, couponGrantId: null, validFrom: from, validUntil: until,
    status: 'active', autoRenewEnabled: false, trafficLimitBytes: '107374182400', resetStrategy: 'MONTH_ROLLING',
    squadUuids: squads.map(s => s.uuid), rolloverMinRemainingBps: 8500, createdAt: at(-5 * day), updatedAt: at(0) }
}
const active = purchase('current', at(-5 * day), at(25 * day))
const queued = { ...purchase('queued', at(25 * day), at(32 * day)), status: 'queued' as const }
let controls: ComboControls = { activePurchase: params.has('inactive') ? null : active, queuedPurchase: params.has('inactive') || params.has('noqueued') ? null : queued, squads: params.has('inactive') ? [] : squads, mutable: !params.has('inactive') && !params.has('blocked'), operation: null }
const receipts = new Map<string, OperationReceipt>()
const state = { loads: 0, additions: 0, additionKeys: [] as string[], switches: 0, activations: 0, polls: 0, forfeited: null as Purchase | null, automation: true }
let preferences: UserPreferences = { activeCombo: !params.has('inactive'), notifications: { combos: true, traffic: true, money: true, activity: true, account: true }, showAroundTx: true, showActivity: true, includeNodePrices: true, showReferralUsername: true }
let tag: GroupMemberTag = { groupJoined: true, editable: true, tag: 'Traveler', reasonCode: null }
async function pause(): Promise<void> {
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 2500))
  if (params.has('error')) throw new Error('Constructed settings load failure')
}
comboControlsApi.get = async () => { state.loads++; await pause(); if (params.has('blocked') && state.loads > 1) controls = { ...controls, mutable: true }; return controls }
comboControlsApi.quoteActivation = async (): Promise<EarlyActivationQuote> => ({ currentPurchaseId: active.id, queuedPurchaseId: queued.id, eligible: controls.mutable, currentValidUntil: active.validUntil, newValidFrom: at(0), newValidUntil: at(7 * day) })
function receipt(kind: string): OperationReceipt {
  state.polls = 0
  const value: OperationReceipt = { id: 'audit-operation-' + (state.switches + state.activations), kind, status: 'queued', createdAt: at(0), updatedAt: at(0) }
  receipts.set(value.id, value)
  controls = { ...controls, mutable: false, operation: value }
  return value
}
comboControlsApi.switchSquad = async (_purchaseId, uuid, enabled) => {
  const enabledCount = squads.filter(s => s.enabled).length
  const target = squads.find(s => s.uuid === uuid)!
  if (!enabled && target.enabled && enabledCount === 1) throw new ApiError(409, { code: 'LAST_SQUAD_REQUIRED', message: '', requestId: 'audit' })
  target.enabled = enabled
  state.switches++
  for (const squad of squads) squad.canDisable = squad.enabled && squads.filter(s => s.enabled).length > 1
  return receipt('member_squad_switch')
}
comboControlsApi.activate = async (_queuedId, _currentId, confirmation) => {
  if (!['ACTIVATE NEXT COMBO', '启用下一套餐'].includes(confirmation.trim())) throw new Error('Constructed confirmation refusal')
  state.activations++
  state.forfeited = { ...active, status: 'cancelled', validUntil: at(0) }
  controls = { ...controls, activePurchase: { ...queued, status: 'activating', validFrom: at(0), validUntil: at(7 * day) }, queuedPurchase: null }
  return receipt('member_early_activation')
}
memberOperationsApi.getOperation = async id => {
  state.polls++
  if (params.has('pollerror')) throw new Error('Constructed polling failure')
  const old = receipts.get(id)!
  const next = { ...old, status: state.polls > 1 ? 'succeeded' as const : 'processing' as const, updatedAt: at(state.polls * 1500) }
  receipts.set(id, next)
  if (next.status === 'succeeded') controls = { ...controls, mutable: true, operation: null, activePurchase: controls.activePurchase ? { ...controls.activePurchase, status: 'active' } : null }
  return next
}
preferencesApi.get = async () => preferences
preferencesApi.update = async body => { preferences = { ...preferences, ...body, notifications: { ...preferences.notifications, ...body.notifications } }; return preferences }
preferencesApi.getTag = async () => tag
preferencesApi.updateTag = async value => { tag = { ...tag, tag: value }; return tag }
memberOperationsApi.getTrafficResetAutomation = async () => ({ enabled: state.automation, updatedAt: at(0) })
memberOperationsApi.updateTrafficResetAutomation = async enabled => { state.automation = enabled; return { enabled, updatedAt: at(0) } }
memberOperationsApi.getPurchaseRefundQuote = async () => ({ purchaseId: active.id, eligible: true, reasonCode: null, refund: active.price, quotedAt: at(0), eligibilityExpiresAt: at(day) })
memberOperationsApi.refundPurchase = async () => receipt('purchase_member_refund')
api.getAutoRenewal = async () => ({ enabled: false, canEnable: true, ineligibleReason: null, purchaseId: active.id, quotedAt: at(0) }) as never
const optionalSquads = [
  { id: 'optional-jp', remnaSquadUuid: 'optional-jp', name: 'Japan transit', country: 'JP', multiplier: 1 },
  { id: 'optional-nl', remnaSquadUuid: 'optional-nl', name: 'Netherlands transit', country: 'NL', multiplier: 0.5 },
].map((squad, index) => ({ ...squad, description: '', profile: { type: 'international_network', countryCode: squad.country, portMbps: null, upstreamCarriers: [] }, price: { currency: 'TXB', minor: String(100 + index * 100), display: '' }, visible: true, upstreamPresent: true, activationRequired: false, stockHeldByCurrentUser: false, stockLimit: null, stockRemaining: null, createdAt: at(0), updatedAt: at(0), accessibleNodes: [{ uuid: 'node-' + squad.id, name: squad.name + ' edge', countryCode: squad.country, consumptionMultiplier: squad.multiplier }], geocheckEnabled: false }))
api.getCatalog = async () => {
  if (params.has('addonslow')) await new Promise(resolve => setTimeout(resolve, 10000))
  if (params.has('addonerror')) throw new Error('Constructed catalog load failure')
  return { combos: [], addons: params.has('addonempty') ? [] : optionalSquads, nodes: [] } as never
}
api.getStatistics = async () => ({ database: { squadByCombo: [{ id: 'current', label: 'Standard', segments: optionalSquads.map((squad, index) => ({ id: squad.id, label: squad.name, value: index ? 20 : 70 })) }] } }) as never
api.quotePurchaseAddons = async (purchaseId, ids) => {
 if (params.has('addonoverride')) throw new ApiError(409, { code: 'SQUAD_ADDITION_OVERRIDDEN', message: '' })
 return ({ purchaseId, addonSquadUuids: ids, price: { currency: 'TXB', minor: '250', display: '2.50 TXB' }, effectiveAt: at(0), expiresAt: active.validUntil }) as never
}
const addedByKey = new Map<string, Purchase>()
let lostAdditionReply = false
api.addPurchaseAddons = async (_purchaseId, ids, key) => {
  if (params.has('addoncommitoverride')) throw new ApiError(409, { code: 'SQUAD_ADDITION_OVERRIDDEN', message: '' })
  state.additionKeys.push(key)
  const replay = addedByKey.get(key)
  if (replay) return replay
  state.additions++
  const updated = { ...active, squadUuids: [...active.squadUuids, ...ids] }
  controls = { ...controls, activePurchase: updated }
  addedByKey.set(key, updated)
  if (params.has('addonlost') && !lostAdditionReply) { lostAdditionReply = true; throw new Error('Constructed lost addition reply') }
  return updated
}
Object.assign(window, { __controlsAudit: state, __controlsAuditLocale: setLocale, __controlsSnapshot: () => controls })
const app = createApp(SettingsAudit, { purchase: active })
app.config.globalProperties.$t = t
app.use(createPinia())
usePreferencesStore().bind('audit-user')
app.use(createRouter({ history: createWebHistory(), routes: [{ path: '/fixtures/settings-audit.html', component: SettingsAudit }] }))
app.use(ui)
app.mount('#app')
