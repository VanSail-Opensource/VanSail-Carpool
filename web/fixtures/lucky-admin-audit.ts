import '@/styles/main.css'
import { createApp } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import { featuresApi } from '@/api/features'
import { adminOperationsApi } from '@/api/adminOperations'
import { t } from '@/i18n'
import LuckyAdminAudit from './LuckyAdminAudit.vue'

const params = new URLSearchParams(location.search)
const now = new Date().toISOString()
const draws = [
  {
    id: 'instant-audit', kind: 'instant', status: 'open', enabled: true,
    name: 'Weekend rewards', description: 'An instant draw for a quiet weekend.',
    feeTxbMinor: '125', expectedParticipation: 100, seats: 0, createdAt: now, updatedAt: now,
    prizes: [
      { id: 'instant-a', name: '5 TXB', probabilityBps: 6500, reward: { kind: 'txb_delta', range: { min: 500, max: 500, distribution: 'uniform' } } },
      { id: 'instant-b', name: '20 TXB', probabilityBps: 3500, reward: { kind: 'txb_delta', range: { min: 2000, max: 2000, distribution: 'uniform' } } },
    ],
  },
  {
    id: 'raffle-audit', kind: 'raffle', status: 'draft', enabled: false,
    name: 'Community weekend draw', description: 'Ten seats, one reward per seat.',
    feeTxbMinor: '200', threshold: 10, keyword: 'weekend', command: 'weekend_draw',
    seats: 0, createdAt: now, updatedAt: now,
    prizes: [
      { id: 'raffle-a', name: '20 TXB', stock: 4, reward: { kind: 'txb_delta', range: { min: 2000, max: 2000, distribution: 'uniform' } } },
      { id: 'raffle-b', name: 'No prize', stock: 6, reward: { kind: 'none' } },
    ],
  },
]
const pause = () => params.has('slow') ? new Promise((resolve) => setTimeout(resolve, 3000)) : Promise.resolve()
featuresApi.getAdminActivityGames = async () => {
  await pause()
  if (params.has('error')) throw new Error('Constructed activity failure')
  return { items: [] } as never
}
featuresApi.getAdminLuckyDraws = async () => {
  await pause()
  if (params.has('error')) throw new Error('Constructed draw failure')
  return { items: params.has('empty') ? [] : draws } as never
}
adminOperationsApi.getCatalogOptions = async () => ({
  combos: [{ id: 'combo-basic', name: 'Basic', price: { minor: '1000', currency: 'TXB', display: '10.00 TXB' }, validityDays: 30 }],
  squads: [{ id: 'squad-1', groupId: 'squad-1', name: 'Europe' }],
}) as never
featuresApi.getAdminLuckyDrawForecast = async () => ({
  entries: 100, incomeMinor: '12500', expectedExpenseMinMinor: '10200', expectedExpenseMaxMinor: '15600',
  possibleExpenseMinMinor: '0', possibleExpenseMaxMinor: '200000', breakEvenMinMinor: '1',
  breakEvenMaxMinor: '2000', averageBalanceMinor: '12000', multiplierUnavailable: false, couponOneTermOnly: false,
}) as never
featuresApi.saveAdminLuckyDraw = async (id, body) => {
  const existing = draws.find((item) => item.id === id)
  const saved = { ...(existing ?? {}), ...body, id: id ?? 'new-draw', status: existing?.status ?? 'draft',
    seats: existing?.seats ?? 0, createdAt: existing?.createdAt ?? now, updatedAt: new Date().toISOString(),
    prizes: body.prizes.map((prize, index) => ({ ...prize, id: prize.id || 'new-prize-' + index })) }
  if (existing) Object.assign(existing, saved)
  else draws.push(saved as typeof draws[number])
  return saved as never
}
featuresApi.publishAdminLuckyDraw = async (id) => {
  const draw = draws.find((item) => item.id === id)
  if (draw) draw.status = 'publishing'
  return undefined as never
}
const auditState = { draws }
;(window as Window & { __audit?: typeof auditState }).__audit = auditState
const app = createApp(LuckyAdminAudit)
app.config.globalProperties.$t = t
app.use(ui)
app.mount('#app')
