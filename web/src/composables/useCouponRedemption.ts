import { getCurrentScope, onScopeDispose, readonly, shallowRef } from 'vue'

import type { CouponRedemption } from '@/api/features'
import { featuresApi } from '@/api/features'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { notifyHaptic } from '@/utils/telegram'

export function useCouponRedemption() {
  const redeeming = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const message = shallowRef<string | null>(null)
  const redemptionKeys = new Map<string, string>()
  let disposed = false

  async function redeem(rawCode: string): Promise<CouponRedemption | null> {
    const code = rawCode.trim().toUpperCase()
    if (disposed || redeeming.value || !code) return null

    redeeming.value = true
    error.value = null
    message.value = null
    try {
      const idempotencyKey = redemptionKeys.get(code) ?? createUuid()
      redemptionKeys.set(code, idempotencyKey)
      const result = await featuresApi.redeemCoupon(code, idempotencyKey)
      if (disposed) return null
      redemptionKeys.delete(code)
      message.value = result.grant
        ? t('coupons.addedToWallet', { name: result.coupon.name })
        : t('coupons.appliedToBalance', { name: result.coupon.name })
      notifyHaptic('success')
      return result
    } catch (caught) {
      if (disposed) return null
      error.value = localizedError(caught, 'errors.couponRedeem')
      notifyHaptic('error')
      return null
    } finally {
      if (!disposed) redeeming.value = false
    }
  }

  if (getCurrentScope()) onScopeDispose(() => { disposed = true; redemptionKeys.clear(); message.value = null; error.value = null; redeeming.value = false })

  return {
    redeeming: readonly(redeeming),
    error: readonly(error),
    message: readonly(message),
    redeem,
  }
}
