import '@/styles/main.css'
import { createApp } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { featuresApi, type ActivityOverview } from '@/api/features'
import { setLocale, t } from '@/i18n'
import { setExperience } from '@/utils/experience'
import LuckyAudit from './LuckyAudit.vue'
import { installInteractionFeedback } from './interaction-feedback'

const auditParams = new URLSearchParams(globalThis.location.search)
setExperience('miniapp')
setLocale(auditParams.has('zh') ? 'zh-CN' : 'en')
const feedback = installInteractionFeedback()
const prizes = Array.from({ length: 20 }, (_, index) => ({ id: `prize-${index + 1}`, name: index === 19 ? 'Annual bonus · 500 TXB' : index % 3 === 0 ? `${index + 1} days added` : `${(index + 1) * 5} TXB` }))
const state = { delay: 1200, reads: 0, bets: 0, betKeys: [] as string[], failNext: false, loadFailure: auditParams.has('error'), selectedId: 'prize-20', rewardKind: 'txb_delta', betOutcome: auditParams.has('loss') ? 'loss' : 'win' }
const drawPrizes = prizes.map((prize) => ({ ...prize }))
const overview = {
  balance: { currency: 'TXB', minor: '10000', display: '100.00 TXB' },
  timeZone: 'Asia/Shanghai', checkedInToday: true, dailyRewardMinTxbMinor: '50', dailyRewardMaxTxbMinor: '100',
  groupBoost: { state: 'boosted', count: 2, boostUrl: 'https://t.me/boost?c=1234567890' },
  games: auditParams.has('bet') ? [{ id: 'bet-1', name: 'Coin flip', icon: 'coin', description: 'Constructed outcome for interaction review.', winChanceBps: 5000, minimumStakeMinor: '100', maximumStakeMinor: '1000', returnMultiplierBps: 20000, enabled: true }] : [], draws: [
    { id: 'draw-1', name: 'Weekend rewards', description: 'One draw, one recorded result.', feeTxbMinor: '100', enabled: true, prizes },
    { id: 'draw-2', name: 'Midnight bonus', description: 'A second draw to audit parallel button states.', feeTxbMinor: '250', enabled: true, prizes: drawPrizes },
  ], recentResults: [],
  groupMessageReward: { enabled: auditParams.has('progress'), localDate: '2026-09-25', messageCount: 12, threshold: 5, rewardMinor: '100', rewarded: true },
} satisfies ActivityOverview
featuresApi.getActivity = async () => {
  state.reads++
  if (auditParams.has('slow')) await new Promise((resolve) => setTimeout(resolve, 800))
  if (state.loadFailure) throw new Error('Constructed activity read failure')
  return structuredClone(auditParams.has('empty') ? { ...overview, draws: [] } : overview)
}
featuresApi.placeBet = async (_gameId, _minor, key) => {
  state.bets++; state.betKeys.push(key)
  await new Promise(resolve => setTimeout(resolve, state.delay))
  if (state.failNext) { state.failNext = false; throw new Error('Constructed network failure') }
  return { id: `bet-${Date.now()}`, kind: 'bet', outcome: state.betOutcome, message: '', reward: { kind: 'none' }, balanceAfter: overview.balance, createdAt: new Date().toISOString() } as never
}
featuresApi.drawLuckyPrize = async (drawId: string) => {
  await new Promise((resolve) => setTimeout(resolve, state.delay))
  if (state.failNext) { state.failNext = false; throw new Error('Constructed network failure') }
  const prize = prizes.find((candidate) => candidate.id === state.selectedId) ?? prizes[19]
  const reward = state.rewardKind === 'none' ? { kind: 'none' } : state.rewardKind === 'negative' ? { kind: 'txb_delta', txbDeltaMinor: '-50' } : { kind: 'txb_delta', txbDeltaMinor: '50000' }
  return { id: `result-${Date.now()}`, kind: 'draw', outcome: 'complete', message: '', drawId, prizeId: prize.id, prizeName: prize.name, reward, balanceAfter: { currency: 'TXB', minor: '59900', display: '599.00 TXB' }, createdAt: new Date().toISOString() } as never
}
const auditState = Object.assign(state, { overview, feedback })
;(window as Window & { __audit?: typeof auditState }).__audit = auditState
const app = createApp(LuckyAudit)
app.config.globalProperties.$t = t
app.use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: LuckyAudit }] })).use(ui)
app.mount('#app')
