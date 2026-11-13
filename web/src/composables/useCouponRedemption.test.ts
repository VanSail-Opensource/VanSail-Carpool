import { effectScope } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

const { redeemCoupon, createUuid, notifyHaptic } = vi.hoisted(() => ({
  redeemCoupon: vi.fn(),
  createUuid: vi.fn(),
  notifyHaptic: vi.fn(),
}))

vi.mock('@/api/features', () => ({ featuresApi: { redeemCoupon } }))
vi.mock('@/i18n', () => ({ localizedError: () => 'localized-error', t: (key: string) => key }))
vi.mock('@/utils/browserCompatibility', () => ({ createUuid }))
vi.mock('@/utils/telegram', () => ({ notifyHaptic }))

import { useCouponRedemption } from './useCouponRedemption'

describe('guided coupon redemption', () => {
  afterEach(() => vi.clearAllMocks())

  it('normalizes a code and returns its wallet grant', async () => {
    createUuid.mockReturnValue('coupon-key')
    redeemCoupon.mockResolvedValue({ coupon: { name: 'North' }, grant: { id: 'grant-1' } })
    const coupon = useCouponRedemption()

    const result = await coupon.redeem(' north ')

    expect(redeemCoupon).toHaveBeenCalledWith('NORTH', 'coupon-key')
    expect(result?.grant?.id).toBe('grant-1')
    expect(coupon.message.value).toBe('coupons.addedToWallet')
    expect(notifyHaptic).toHaveBeenCalledWith('success')
  })

  it('retains the idempotency key after an ambiguous redemption failure', async () => {
    createUuid.mockReturnValue('coupon-key')
    redeemCoupon
      .mockRejectedValueOnce(new Error('network'))
      .mockResolvedValueOnce({ coupon: { name: 'North' }, grant: null })
    const coupon = useCouponRedemption()

    await coupon.redeem('north')
    await coupon.redeem('NORTH')

    expect(redeemCoupon).toHaveBeenNthCalledWith(1, 'NORTH', 'coupon-key')
    expect(redeemCoupon).toHaveBeenNthCalledWith(2, 'NORTH', 'coupon-key')
    expect(coupon.error.value).toBeNull()
    expect(coupon.message.value).toBe('coupons.appliedToBalance')
    expect(notifyHaptic).toHaveBeenLastCalledWith('success')
  })

  it('does not create a request for an empty code', async () => {
    const coupon = useCouponRedemption()

    await expect(coupon.redeem('   ')).resolves.toBeNull()
    expect(redeemCoupon).not.toHaveBeenCalled()
  })
  it('does not publish a redeemed grant or haptics after its owner scope is retired', async () => {
    let finish!: (value: unknown) => void
    redeemCoupon.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    createUuid.mockReturnValue('retired-coupon-key')
    const scope = effectScope()
    const coupon = scope.run(useCouponRedemption)!
    const pending = coupon.redeem('north')
    scope.stop()
    finish({ coupon: { name: 'North' }, grant: { id: 'old-owner-grant' } })
    expect(await pending).toBeNull()
    expect(await coupon.redeem('new-code')).toBeNull()
    expect(redeemCoupon).toHaveBeenCalledOnce()
    expect(notifyHaptic).not.toHaveBeenCalled()
    expect(coupon.message.value).toBeNull()
  })

})
