import { effectScope } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { featuresApi, type AffiliateReferralPage } from '@/api/features'
import { useAffiliateCentre } from './useAffiliateCentre'

afterEach(() => vi.restoreAllMocks())

const rows = (page: number): AffiliateReferralPage => ({ page, pageSize: 5, total: 15, totalPages: 3, items: [] })

describe('affiliate referral navigation', () => {
  it('keeps the latest requested page when responses arrive out of order', async () => {
    let finishSecond!: (value: AffiliateReferralPage) => void
    vi.spyOn(featuresApi, 'getAffiliateReferrals')
      .mockImplementationOnce(() => new Promise(resolve => { finishSecond = resolve }))
      .mockResolvedValueOnce(rows(3))
    const scope = effectScope()
    const centre = scope.run(useAffiliateCentre)!
    const second = centre.setPage(2)
    await centre.setPage(3)
    finishSecond(rows(2))
    await second
    expect(centre.referrals.value?.page).toBe(3)
    expect(centre.page.value).toBe(3)
    expect(centre.referralsLoading.value).toBe(false)
    scope.stop()
  })

  it('allows the failed page to be requested again and clears its error', async () => {
    const read = vi.spyOn(featuresApi, 'getAffiliateReferrals')
      .mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(rows(2))
    const scope = effectScope()
    const centre = scope.run(useAffiliateCentre)!
    await centre.setPage(2)
    expect(centre.error.value).toBeTruthy()
    await centre.setPage(2)
    expect(read).toHaveBeenCalledTimes(2)
    expect(centre.referrals.value?.page).toBe(2)
    expect(centre.error.value).toBeNull()
    scope.stop()
  })
})
