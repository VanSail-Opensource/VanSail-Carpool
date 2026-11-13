import type { CouponDefinition } from '@/api/features'
import { txbInputFromMinor } from '@/utils/format'

export interface CouponDraft {
  code: string; name: string; kind: CouponDefinition['kind']; discountMode: 'fixed' | 'percent'
  valueTxb: string; percent: string | number; factor: string | number; capTxb: string
  globalLimit: string; perUserLimit: string; eligibleComboIds: string[]; eligibleSquadIds: string[]
  expiresAt: string; active: boolean
}

const maxMinor = 9_223_372_036_854_775_807n
function scaledPositive(value: string | number, places: number): string | null {
  const match = /^(\d+)(?:\.(\d*))?$/.exec(String(value).trim())
  if (!match || (match[2]?.length ?? 0) > places) return null
  const result = BigInt(match[1]!) * 10n ** BigInt(places) + BigInt((match[2] ?? '').padEnd(places, '0'))
  return result > 0n && result <= maxMinor ? result.toString() : null
}
function optionalLimit(value: string): number | null | undefined {
  if (!value.trim()) return null
  if (!/^\d+$/.test(value.trim())) return undefined
  const limit = Number(value)
  return Number.isSafeInteger(limit) && limit > 0 ? limit : undefined
}

export function couponDraft(coupon?: CouponDefinition | null): CouponDraft {
  return coupon ? {
    code: coupon.code, name: coupon.name, kind: coupon.kind, discountMode: coupon.discountMode ?? 'fixed',
    valueTxb: (['purchase_once', 'purchase_recurring'].includes(coupon.kind) && coupon.discountMode === 'percent') || coupon.kind === 'balance_multiply' ? '5.00' : txbInputFromMinor(coupon.valueMinorOrBps),
    percent: coupon.discountMode === 'percent' ? txbInputFromMinor(coupon.valueMinorOrBps) : '10.00',
    factor: coupon.kind === 'balance_multiply' ? (BigInt(coupon.valueMinorOrBps) / 10000n).toString() + '.' + (BigInt(coupon.valueMinorOrBps) % 10000n).toString().padStart(4, '0') : '2.0000',
    capTxb: coupon.percentCapMinor ? txbInputFromMinor(coupon.percentCapMinor) : '',
    globalLimit: coupon.globalUseLimit === null ? '' : String(coupon.globalUseLimit),
    perUserLimit: coupon.perUserUseLimit === null ? '' : String(coupon.perUserUseLimit),
    eligibleComboIds: [...coupon.eligibleComboIds], eligibleSquadIds: [...coupon.eligibleSquadIds],
    expiresAt: coupon.expiresAt?.slice(0, 10) ?? '', active: coupon.active,
  } : {
    code: '', name: '', kind: 'purchase_once', discountMode: 'percent', valueTxb: '5.00', percent: '10.00',
    factor: '2.0000', capTxb: '', globalLimit: '', perUserLimit: '1', eligibleComboIds: [], eligibleSquadIds: [], expiresAt: '', active: true,
  }
}

export function couponPayload(draft: CouponDraft, original: CouponDefinition | null): Partial<CouponDefinition> | null {
  const purchase = draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring'
  const percentage = purchase && draft.discountMode === 'percent'
  const valueMinorOrBps = scaledPositive(percentage ? draft.percent : draft.kind === 'balance_multiply' ? draft.factor : draft.valueTxb, draft.kind === 'balance_multiply' ? 4 : 2)
  const percentCapMinor = percentage && draft.capTxb.trim() ? scaledPositive(draft.capTxb, 2) : null
  const globalUseLimit = optionalLimit(draft.globalLimit)
  const perUserUseLimit = optionalLimit(draft.perUserLimit)
  const code = draft.code.trim().toUpperCase(), name = draft.name.trim()
  if (!valueMinorOrBps || !/^[A-Z0-9_-]{1,40}$/.test(code) || !name || name.length > 80 || globalUseLimit === undefined || perUserUseLimit === undefined) return null
  if ((percentage && BigInt(valueMinorOrBps) > 10000n) || (draft.kind === 'balance_multiply' && (BigInt(valueMinorOrBps) <= 10000n || BigInt(valueMinorOrBps) > 1000000n))) return null
  if ((percentage && draft.capTxb.trim() && !percentCapMinor) || draft.eligibleComboIds.length > 500 || draft.eligibleSquadIds.length > 500) return null
  let expiresAt = original?.expiresAt ?? null
  if (draft.expiresAt !== (original?.expiresAt?.slice(0, 10) ?? '')) {
    if (!draft.expiresAt) expiresAt = null
    else {
      const date = new Date(draft.expiresAt + 'T23:59:59Z')
      if (!/^\d{4}-\d{2}-\d{2}$/.test(draft.expiresAt) || !Number.isFinite(date.getTime()) || date.toISOString().slice(0, 10) !== draft.expiresAt) return null
      expiresAt = date.toISOString()
    }
  }
  const values: Partial<CouponDefinition> = {
    code, name, kind: draft.kind, discountMode: purchase ? draft.discountMode : undefined,
    valueMinorOrBps, percentCapMinor, eligibleComboIds: purchase ? [...draft.eligibleComboIds] : [],
    eligibleSquadIds: purchase ? [...draft.eligibleSquadIds] : [], expiresAt,
    active: draft.active, globalUseLimit, perUserUseLimit,
  }
  if (!original) return values
  return Object.fromEntries(Object.entries(values).filter(([key, value]) => JSON.stringify(value) !== JSON.stringify(original![key as keyof CouponDefinition]))) as Partial<CouponDefinition>
}

