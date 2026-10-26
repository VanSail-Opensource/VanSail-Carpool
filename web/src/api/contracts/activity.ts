import type { Money, RFC3339 } from '../types'

export interface ValueRange {
  min: number
  max: number
  distribution: 'uniform' | 'gaussian' | 'power_law'
  positiveChanceBps?: number
}

export type RewardKind = 'none' | 'txb_delta' | 'coupon_grant' | 'subscription_extension'
  | 'entitlement_grant' | 'squad_access' | 'core_combo_switch' | 'traffic_grant'
  | 'traffic_reset' | 'balance_multiplier' | 'coupon_recurring' | 'coupon_once'

export interface Reward {
  kind: RewardKind
  txbDeltaMinor?: string
  couponId?: string
  extensionDays?: number
  range?: ValueRange
  resolvedValue?: number
  comboId?: string
  squadUuids?: readonly string[]
  renewalPriceMinor?: number
  trafficLimitBytes?: number
  includeInRenewal?: boolean
  discountMode?: 'fixed' | 'percent'
}

export interface BetGame {
  id: string
  name: string
  icon: string
  description: string
  winChanceBps: number
  minimumStakeMinor: string
  maximumStakeMinor: string
  returnMultiplierBps: number
  enabled: boolean
}

export interface LuckyDraw {
  id: string
  name: string
  description: string
  feeTxbMinor: string
  enabled: boolean
  prizes: readonly LuckyDrawPrizePreview[]
}

export interface LuckyDrawPrizePreview {
  id: string
  name: string
}

export interface ActivitySettings {
  timezone: string
  dailyRewardMinTxb: string
  dailyRewardMinTxbMinor: string
  dailyRewardMaxTxb: string
  dailyRewardMaxTxbMinor: string
  groupMessageThreshold: number
  groupMessageRewardTxb: string
  groupMessageRewardTxbMinor: string
}

export interface ActivitySettingsWrite {
  timezone: string
  dailyRewardMinTxb: string
  dailyRewardMaxTxb: string
  groupMessageThreshold: number
  groupMessageRewardTxb: string
}

export interface GroupMessageRewardStatus {
  enabled: boolean
  localDate: string
  messageCount: number
  threshold: number
  rewardMinor: string
  rewarded: boolean
  rewardedAt?: RFC3339
}

export interface LuckyDrawPrize {
  id: string
  name: string
  probabilityBps?: number
  stock?: number
  reward: Reward
}

export interface LuckyDrawAdmin extends LuckyDraw {
  kind: 'instant' | 'raffle'
  status: 'draft' | 'publishing' | 'open' | 'settling' | 'completed' | 'cancelled'
  expectedParticipation?: number
  threshold?: number
  keyword?: string
  command?: string
  seats: number
  prizes: LuckyDrawPrize[]
  createdAt: RFC3339
  updatedAt: RFC3339
}

export interface LuckyDrawWrite {
  name: string
  description: string
  enabled: boolean
  kind: 'instant' | 'raffle'
  feeTxbMinor: string
  expectedParticipation: number
  threshold: number
  keyword: string
  command: string
  prizes: LuckyDrawPrize[]
}

export interface LuckyDrawForecast {
  entries: number
  incomeMinor: string
  expectedExpenseMinMinor: string | null
  expectedExpenseMaxMinor: string | null
  possibleExpenseMinMinor: string | null
  possibleExpenseMaxMinor: string | null
  breakEvenMinMinor: string | null
  breakEvenMaxMinor: string | null
  averageBalanceMinor: string | null
  multiplierUnavailable: boolean
  couponOneTermOnly: boolean
}

export interface ActivityResult {
  id: string
  kind: 'check_in' | 'bet' | 'draw'
  outcome: 'win' | 'loss' | 'complete'
  message: string
  reward: Reward
  stakeTxbMinor?: string
  drawId?: string
  prizeId?: string
  prizeName?: string
  balanceAfter: Money
  createdAt: RFC3339
}

export interface ActivityOverview {
  groupBoost: GroupBoostStatus
  balance: Money
  timeZone: string
  checkedInToday: boolean
  dailyRewardMinTxbMinor: string
  dailyRewardMaxTxbMinor: string
  games: BetGame[]
  draws: LuckyDraw[]
  recentResults: ActivityResult[]
  groupMessageReward: GroupMessageRewardStatus
}

export interface GroupBoostStatus {
  state: 'required' | 'boosted' | 'unavailable'
  count: number | null
  boostUrl: string | null
}
