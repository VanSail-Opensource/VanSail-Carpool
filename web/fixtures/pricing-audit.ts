import { createApp } from 'vue'
import { installPricingCheckout } from './pricing-checkout'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme, setBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import type { PricingPolicy } from '@/api/pricing'
import { createPinia } from 'pinia'
import { personalForecast, userPrices } from './pricing-data'
import PricingAudit from './PricingAudit.vue'
import '@/styles/main.css'
import '@/browser/styles.css'

setExperience(new URLSearchParams(location.search).get('view') === 'member' || new URLSearchParams(location.search).get('view') === 'checkout' ? 'miniapp' : 'browser')
initializeBrowserTheme()
const params = new URLSearchParams(location.search)
if (params.has('light')) setBrowserTheme('light')
if (params.has('dark')) setBrowserTheme('dark')
const view = params.get('view') ?? 'policy'
setLocale(params.has('zh') ? 'zh-CN' : 'en')
let policy: PricingPolicy = {
  productType: 'combo', productId: 'audit-combo', dynamicEnabled: false,
  purchasePricingMode: 'FIXED', renewalPricingMode: 'FIXED', personalDiscountsEnabled: false,
  fixedPriceMinor: '1000', maxIncreaseBps: 0, maxDiscountBps: 0,
  overrideStatus: 'NONE', lastApprovedPriceVersion: '', version: 0, updatedBy: '', updatedAt: new Date().toISOString(),
}
const now = new Date()
const health = [
  { nodeId: 'node-tokyo', state: 'HEALTHY', cpu: 31.2, memory: 42.1, at: now.toISOString(), reportedAt: now.toISOString(), since: now.toISOString() },
  { nodeId: 'node-frankfurt', state: 'OVERLOADED', cpu: 98.5, memory: 84.2, at: now.toISOString(), reportedAt: now.toISOString(), since: now.toISOString() },
  { nodeId: 'node-monitoring', state: 'TELEMETRY_STALE', cpu: 0, memory: 0, at: now.toISOString(), reportedAt: now.toISOString(), since: now.toISOString() },
]
const calls: string[] = []
Object.assign(window, { pricingAuditCalls: calls })
if (view === 'member') policy = { ...policy, dynamicEnabled: true, renewalPricingMode: 'DYNAMIC', version: 1 }
window.fetch = async (input, init) => {
  const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin)
  ;(window as unknown as { pricingAuditQueries: string[] }).pricingAuditQueries ??= []
  ;(window as unknown as { pricingAuditQueries: string[] }).pricingAuditQueries.push(url.pathname + url.search)
  calls.push(`${init?.method ?? 'GET'} ${url.pathname}`)
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 2500))
  if (params.has('error') || init?.method === 'PUT' && params.has('save-error')) return new Response(JSON.stringify({ code: 'PRICING_PRICE_VERSION_CONFLICT', message: 'Conflict' }), { status: 409, headers: { 'Content-Type': 'application/json' } })
  if (url.pathname.endsWith('/predictions')) return new Response(JSON.stringify(personalForecast(params)), { headers: { 'Content-Type':'application/json' } })
  if (url.pathname.endsWith('/prices')) return new Response(JSON.stringify(userPrices(params,url.searchParams.get('cursor'))), { headers: { 'Content-Type':'application/json' } })
  let value: unknown = policy
  if (url.pathname.endsWith('/admin/combos')) value={items:[]}
  else if (url.pathname.endsWith('/admin/squad-products')) value={items:['normal','hidden','unavailable','missing'].map((status,index)=>({id:`s${index}`,groupId:`g${index}`,name:`PAYG ${status}`,description:'',profile:null,billingMode:'payg',paygState:status==='missing'?'normal':status,price:{currency:'TXB',minor:'0',display:'0.00 TXB'},paygPrice:{currency:'TXB',minor:'50',display:'0.50 TXB'},visible:status!=='hidden',upstreamPresent:status!=='missing',stockLimit:null,stockRemaining:null,stockHeldByCurrentUser:false,activationRequired:false,accessibleNodes:[],createdAt:now.toISOString(),updatedAt:now.toISOString()}))}
  else if (url.pathname.endsWith('/pricing/health')) value = { health: params.has('empty') ? [] : health, observedAt: now.toISOString() }
  else if (url.pathname.endsWith('/admin/pricing')) value = { products: params.has('empty') ? [] : [{ id: 'combo:audit-combo', productType: 'combo', productId: 'audit-combo', name: 'Tokyo monthly', fixedPriceMinor: '1000', policy }, { id: 'payg:audit-payg', productType: 'payg', productId: 'audit-payg', name: 'Frankfurt PAYG', fixedPriceMinor: '150', policy: { ...policy, productType: 'payg', productId: 'audit-payg', dynamicEnabled: true, purchasePricingMode: 'DYNAMIC' } }], nextCursor: '', health: params.has('empty') ? [] : health, storage: { pricingBytes: '15360000', databaseBytes: '314572800', pricingBudgetBytes: '734003200', databaseTargetBytes: '3000000000', pressure: false }, datasets: [], summary: { netRevenueMinor: '238000', recordedCostMinor: null, recordedMarginMinor: null, forecastRevenueMinor: '8300', forecastMarginMinor: null, pendingJobs: 2, failedJobs: 0, interventions30: 3, dynamicProducts: 1 } }
  else if (url.pathname.endsWith('/pricing/data')) {
    const dataset = url.searchParams.get('dataset')
    const data: Record<string, unknown[]> = {
      'forecasts/model_versions': [{ id: 'empirical-daily-v1', calibration_status: 'PRIOR_ONLY', watermark: now.toISOString(), diagnostics_json: '{"samples":0,"validated":false}' }],
      'operations/health_incidents': [{ id: 'incident-audit', scope_id: 'node-frankfurt', state: 'OVERLOADED', started_at: new Date(Date.now() - 2 * 86400000).toISOString(), ended_at: null, reason: 'Sustained average CPU saturation', acknowledged_by: null }],
      'operations/admin_overrides': [{ id: 'override-audit', scope_id: 'combo:audit-combo', command: 'MANUAL_RATE', status: 'ACTIVE', created_at: now.toISOString(), created_by: '99102', reason: 'Campaign contribution review' }],
      'validation/calibration': [{ day: now.toISOString().slice(0, 10), model_version: 'purchase-prior-v1', kind: 'purchase', bucket: 5, samples: '20', probability_sum: 10, observed_sum: '8', brier_sum: 5 }],
      'pricing/eligible_offers': [{ product_type: 'combo', product_id: 'audit-combo', public_minor: '1000', eligible_minor: '950', policy_version: '1', state: 'AVAILABLE', trace_json: '{"readOnly":true,"couponPriority":true}' }],
      'pricing/contracts': [{ purchase_id: 'purchase-audit', contractual_price_minor: '1000', price_limit_minor: '1000', policy_version: '1', recorded_at: now.toISOString(), mode: 'DYNAMIC' }],
      'pricing/pricing_decisions': [{ selected_minor: '1000', model_version: 'response-audit', state: 'SHADOW', generated_at: now.toISOString(), trace_json: JSON.stringify({ candidates: [{ priceMinor: '900', probability: .6, contributionMinor: 240, eligible: false }, { priceMinor: '1000', probability: .5, contributionMinor: 250, eligible: false }, { priceMinor: '1100', probability: .4, contributionMinor: 220, eligible: false }], calibrationStatus: 'UNVALIDATED' }) }],
      'pricing/provider_costs': [{ node_id: 'node-tokyo', cycle_cost_minor: '30000', cycle_days: '30', capacity_bytes: '3000000000000', effective_at: new Date(Date.now() - 45 * 86400000).toISOString(), version: '1' }],
      'pricing/published_prices': [{ id: 'price-audit', price_minor: '1000', source: 'FIXED', model_version: 'fixed-v1', policy_version: '1', created_at: now.toISOString(), valid_until: new Date(Date.now() + 86400000).toISOString() }],
      'forecasts/renewal_predictions': [{ id: 'quote-audit', purchase_id: 'purchase-audit', user_id: '99101', price_minor: '1000', probability: 0.5, contribution: 300, state: 'DEFERRED_HEALTH', health_state: 'OVERLOADED', generated_at: now.toISOString(), settle_after: now.toISOString(), valid_until: new Date(Date.now() + 86400000).toISOString(), trace_json: '{"calibrationStatus":"PRIOR_ONLY","priceLimitMinor":"1100"}' }],
    }
    value = { items: params.has('empty') ? [] : data[dataset ?? ''] ?? [], limit: 100, truncated: params.has('pages') && !url.searchParams.get('cursor'), nextCursor: params.has('pages') && !url.searchParams.get('cursor') ? 'fixture-next' : '' }
  } else if (url.pathname.endsWith('/pricing/history')) {
    const monetary = url.searchParams.get('metric') === 'gross_revenue_minor'
    const factor = monetary ? 100 : 1073741824
    const items = Array.from({ length: 7 }, (_, i) => {
      const target = new Date(Date.now() + (i - 5) * 86400000)
      return { forecastId: `forecast-${i}`, entityType: 'node', entityId: 'node-tokyo', metric: 'traffic_bytes', generatedAt: new Date(target.getTime() - 86400000).toISOString(), targetFrom: target.toISOString(), targetUntil: new Date(target.getTime() + 86400000).toISOString(), horizonSeconds: 86400, predictedP10: (40 + i) * factor, predictedP50: (58 + i) * factor, predictedP90: (74 + i) * factor, predictedMean: (59 + i) * factor, modelVersion: monetary ? 'empirical-receipts-v1' : 'empirical-daily-v1', featureWatermark: new Date(target.getTime() - 86400000).toISOString(), featureSnapshotHash: 'audit', dataCoverageBps: 9666, healthState: 'HEALTHY', calibrationStatus: 'UNVALIDATED', actual: i < 3 ? (61 + i) * factor : null, status: i < 3 ? 'COMPLETE' : i === 3 ? 'MISSING' : i === 4 ? 'CENSORED' : 'PENDING' }
    })
    value = { items: params.has('empty') ? [] : items.reverse(), metrics: { samples: 3, excluded: 4, mae: 3 * factor, medianAbsoluteError: 3 * factor, wape: .05, bias: -3 * factor, coverage: 1, intervalWidth: 32 * factor, pinballLoss: factor } }
  } else if (url.pathname.endsWith('/pricing/interventions/version')) value = { version: policy.version }
  else if (url.pathname.endsWith('/pricing/interventions')) value = { id: 'intervention-created', status: 'ACTIVE' }
  else if (/\/purchases\/[^/]+\/pricing$/.test(url.pathname)) value = { contract: null, quote: { id: 'quote-audit', purchaseId: 'purchase-audit', contractId: 'basis', priceMinor: '1000', policyVersion: 1, contractVersion: 1, state: 'AUTHORIZED', generatedAt: now.toISOString(), authorizedAt: now.toISOString(), settleAfter: new Date(Date.now() + 86400000).toISOString(), validUntil: new Date(Date.now() + 4 * 86400000).toISOString(), settledPurchaseId: null, probability: .5, contribution: 0, healthState: 'HEALTHY', modelVersion: 'renewal-prior-v1', trace: '{}' }, policy, graceSeconds: 259200 }
  if (url.pathname.endsWith('/pricing/exports/authorization')) value = { token: 'fixture-grant', expiresAt: new Date(Date.now() + 300000).toISOString() }
  if (url.pathname.endsWith('/pricing/exports')) {
    const zip = new Uint8Array(22); zip.set([0x50, 0x4b, 0x05, 0x06]);
    const stream = new ReadableStream<Uint8Array>({ async start(controller) { if (params.has('stream-slow')) await new Promise(resolve => setTimeout(resolve, 5000)); try { controller.enqueue(zip); controller.close() } catch { /* Cancelled fixture stream. */ } } })
    return new Response(stream, { headers: { 'Content-Type': 'application/zip' } })
  }
  if (init?.method === 'PUT') {
    const body = JSON.parse(String(init.body)) as { policy: PricingPolicy }
    if (body.policy) { policy = { ...body.policy, version: policy.version + 1 }; value = policy }
  }
  return new Response(JSON.stringify(value), { headers: { 'Content-Type': 'application/json' } })
}
const router = createRouter({ history: createMemoryHistory(), routes: [
  { path: '/admin/pricing', component: PricingAudit, meta: { pricingMode: 'overview' } },
  { path: '/admin/pricing/operations', component: PricingAudit, meta: { pricingMode: 'operations' } },
  { path: '/admin/pricing/exports', component: PricingAudit, meta: { pricingMode: 'exports' } },
  { path: '/admin/pricing/products/:productId', component: PricingAudit, meta: { pricingMode: 'product' } },
  { path: '/admin/users/:userId/pricing', component: PricingAudit, meta: { pricingMode: 'user' } },
] })
await router.push(view === 'product' ? '/admin/pricing/products/combo:audit-combo' : view === 'user' ? '/admin/users/99101/pricing' : view === 'operations' || view === 'exports' ? `/admin/pricing/${view}` : '/admin/pricing')
if (view === 'checkout') installPricingCheckout(params)
const app=createApp(PricingAudit, { view }).use(createPinia()).use(router).use(ui)
app.config.globalProperties.$t=t
app.mount('#app')
