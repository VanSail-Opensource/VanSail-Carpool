import type { RFC3339 } from '../types'

export interface CouponDefinition {
  id: string
  code: string
  name: string
  kind: 'purchase_recurring' | 'purchase_once' | 'balance_add' | 'balance_multiply'
  discountMode?: 'fixed' | 'percent'
  valueMinorOrBps: string
  percentCapMinor?: string | null
  eligibleComboIds: readonly string[]
  eligibleSquadIds: readonly string[]
  expiresAt: RFC3339 | null
  globalUseLimit: number | null
  perUserUseLimit: number | null
  active: boolean
  usageCount: number
  createdAt: RFC3339
  updatedAt: RFC3339
}

export interface CouponGrant {
  id: string
  sourceType: string
  sourceId: string
  status: 'active' | 'consumed' | 'expired'
  useCount: number
  createdAt: RFC3339
  consumedAt: RFC3339 | null
  coupon: CouponDefinition
}

export interface CouponRedemption {
  id: string
  coupon: CouponDefinition
  grant: CouponGrant | null
  balanceDeltaMinor: string
  balanceAfterMinor: string
  idempotencyKey: string
  replayed: boolean
  createdAt: RFC3339
}
