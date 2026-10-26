import type { LuckyDrawAdmin, LuckyDrawPrize, LuckyDrawWrite, Reward, RewardKind, ValueRange } from '@/api/features'
import { createUuid } from '@/utils/browserCompatibility'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'

export interface PrizeDraft {
  id: string
  clientId: string
  name: string
  probability: string
  stock: string
  kind: RewardKind
  rangeMin: string
  rangeMax: string
  distribution: ValueRange['distribution']
  positiveChance: string
  comboId: string
  squadUuids: string[]
  renewalPrice: string
  trafficLimitGiB: string
  includeInRenewal: boolean
  discountMode: 'fixed' | 'percent'
}

export interface DrawDraft {
  kind: 'instant' | 'raffle'
  name: string
  description: string
  fee: string
  enabled: boolean
  expectedParticipation: number
  threshold: number
  keyword: string
  command: string
  prizes: PrizeDraft[]
}

export function blankPrize(): PrizeDraft {
  return {
    id: '', clientId: createUuid(), name: '', probability: '100.00', stock: '1', kind: 'none',
    rangeMin: '1', rangeMax: '10', distribution: 'uniform', positiveChance: '50.00',
    comboId: '', squadUuids: [], renewalPrice: '1.00', trafficLimitGiB: '1',
    includeInRenewal: false, discountMode: 'fixed',
  }
}

export function draftFromDraw(draw: LuckyDrawAdmin | null): DrawDraft {
  return {
    kind: draw?.kind ?? 'instant', name: draw?.name ?? '', description: draw?.description ?? '',
    fee: txbInputFromMinor(draw?.feeTxbMinor ?? '100'), enabled: draw?.enabled ?? true,
    expectedParticipation: draw?.expectedParticipation ?? 100, threshold: draw?.threshold ?? 10,
    keyword: draw?.keyword ?? '', command: draw?.command ?? '',
    prizes: draw?.prizes.length ? draw.prizes.map(prizeFromSaved) : [blankPrize()],
  }
}

function displayRange(reward: Reward, value: number): string {
  if (reward.kind === 'txb_delta' || (reward.kind.startsWith('coupon_') && reward.discountMode === 'fixed')) return (value / 100).toFixed(2)
  if (reward.kind === 'balance_multiplier') return (value / 10000).toFixed(4)
  if (reward.kind.startsWith('coupon_') && reward.discountMode === 'percent') return (value / 100).toFixed(2)
  return String(value)
}

function prizeFromSaved(prize: LuckyDrawPrize): PrizeDraft {
  const draft = blankPrize()
  draft.id = prize.id
  draft.clientId = prize.id || createUuid()
  draft.name = prize.name
  draft.probability = ((prize.probabilityBps ?? 10000) / 100).toFixed(2)
  draft.stock = String(prize.stock ?? 1)
  draft.kind = prize.reward.kind
  draft.rangeMin = displayRange(prize.reward, prize.reward.range?.min ?? 1)
  draft.rangeMax = displayRange(prize.reward, prize.reward.range?.max ?? 10)
  draft.distribution = prize.reward.range?.distribution ?? 'uniform'
  draft.positiveChance = ((prize.reward.range?.positiveChanceBps ?? 5000) / 100).toFixed(2)
  draft.comboId = prize.reward.comboId ?? ''
  draft.squadUuids = [...(prize.reward.squadUuids ?? [])]
  draft.renewalPrice = txbInputFromMinor(String(prize.reward.renewalPriceMinor ?? 100))
  draft.trafficLimitGiB = String((prize.reward.trafficLimitBytes ?? (2 ** 30)) / (2 ** 30))
  draft.includeInRenewal = prize.reward.includeInRenewal ?? false
  draft.discountMode = prize.reward.discountMode ?? 'fixed'
  return draft
}

function scaled(value: string, factor: number): number | null {
  const parsed = Number(value)
  const ticks = Math.round(parsed * factor)
  return Number.isFinite(parsed) && Number.isSafeInteger(ticks) && Math.abs(parsed * factor - ticks) < 0.000001 ? ticks : null
}

function rewardFromDraft(prize: PrizeDraft): Reward | null {
  if (prize.kind === 'none' || prize.kind === 'traffic_reset') return { kind: prize.kind }
  if (prize.kind === 'entitlement_grant') {
    const renewalPriceMinor = moneyFromTxbInput(prize.renewalPrice)
    const gib = Number(prize.trafficLimitGiB)
    if (!prize.comboId || renewalPriceMinor === '' || !Number.isSafeInteger(gib) || gib <= 0 || gib > 1000000) return null
    return { kind: prize.kind, comboId: prize.comboId, squadUuids: prize.squadUuids,
      renewalPriceMinor: Number(renewalPriceMinor), trafficLimitBytes: gib * 2 ** 30 }
  }
  if (prize.kind === 'squad_access') return prize.squadUuids.length ? { kind: prize.kind, squadUuids: prize.squadUuids } : null
  if (prize.kind === 'core_combo_switch') return prize.comboId ? { kind: prize.kind, comboId: prize.comboId } : null
  const factor = prize.kind === 'txb_delta' || (prize.kind.startsWith('coupon_') && prize.discountMode === 'fixed') ? 100
    : prize.kind === 'balance_multiplier' ? 10000
      : prize.kind.startsWith('coupon_') && prize.discountMode === 'percent' ? 100 : 1
  const min = scaled(prize.rangeMin, factor)
  const max = scaled(prize.rangeMax, factor)
  if (min === null || max === null || min > max) return null
  const crossing = min < (prize.kind === 'balance_multiplier' ? 10000 : 0)
    && max > (prize.kind === 'balance_multiplier' ? 10000 : 0)
  const positiveChanceBps = crossing ? scaled(prize.positiveChance, 100) : null
  if (crossing && (positiveChanceBps === null || positiveChanceBps < 0 || positiveChanceBps > 10000)) return null
  const range: ValueRange = { min, max, distribution: prize.distribution }
  if (crossing && positiveChanceBps !== null) range.positiveChanceBps = positiveChanceBps
  const reward: Reward = { kind: prize.kind, range }
  if (prize.kind === 'traffic_grant') reward.includeInRenewal = prize.includeInRenewal
  if (prize.kind === 'coupon_once' || prize.kind === 'coupon_recurring') reward.discountMode = prize.discountMode
  return reward
}

export function serializeDraw(draft: DrawDraft): LuckyDrawWrite | null {
  const feeTxbMinor = moneyFromTxbInput(draft.fee)
  if (!draft.name.trim() || !feeTxbMinor || BigInt(feeTxbMinor) <= 0n || !draft.prizes.length) return null
  if (draft.kind === 'instant' && (!Number.isInteger(draft.expectedParticipation) || draft.expectedParticipation <= 0)) return null
  const reservedCommands = new Set(['start', 'signin', 'sub', 'balance', 'mycombo', 'deduct'])
  if (draft.kind === 'raffle' && (!Number.isInteger(draft.threshold) || draft.threshold <= 0
    || !draft.keyword.trim() || !/^[a-z0-9_]{1,32}$/.test(draft.command)
    || reservedCommands.has(draft.command))) return null
  const prizes: LuckyDrawPrize[] = []
  for (const prize of draft.prizes) {
    const reward = rewardFromDraft(prize)
    const probabilityBps = scaled(prize.probability, 100)
    const stock = Number(prize.stock)
    if (!prize.name.trim() || !reward) return null
    if (draft.kind === 'instant' && (probabilityBps === null || probabilityBps <= 0)) return null
    if (draft.kind === 'raffle' && (!Number.isSafeInteger(stock) || stock <= 0)) return null
    prizes.push({ id: prize.id, name: prize.name.trim(), reward,
      ...(draft.kind === 'instant' ? { probabilityBps: probabilityBps! } : { stock }) })
  }
  if (draft.kind === 'instant' && prizes.reduce((sum, prize) => sum + (prize.probabilityBps ?? 0), 0) !== 10000) return null
  if (draft.kind === 'raffle' && prizes.reduce((sum, prize) => sum + (prize.stock ?? 0), 0) !== draft.threshold) return null
  return { name: draft.name.trim(), description: draft.description.trim(), feeTxbMinor,
    kind: draft.kind, enabled: draft.kind === 'instant' && draft.enabled,
    expectedParticipation: draft.kind === 'instant' ? draft.expectedParticipation : 0,
    threshold: draft.kind === 'raffle' ? draft.threshold : 0,
    keyword: draft.kind === 'raffle' ? draft.keyword.trim() : '',
    command: draft.kind === 'raffle' ? draft.command.trim() : '', prizes }
}
