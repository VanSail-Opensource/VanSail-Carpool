import { afterEach, describe, expect, it } from 'vitest'
import { setLocale, t } from '@/i18n'
import { usageAmount } from '@/utils/usageRational'
import { formatUsageCost } from './usageBreakdownFormat'

afterEach(() => setLocale('en'))

describe('presentation-only rational rounding', () => {
  it.each(['en', 'zh-CN'] as const)('does not describe a positive sub-minor amount as zero in %s', locale => {
    setLocale(locale)
    expect(formatUsageCost(usageAmount(1n, 3n))).toBe(t('usageBreakdown.lessThanAmount', { amount: '0.01 TXB' }))
    expect(formatUsageCost(usageAmount(0n))).toBe('0.00 TXB')
  })
  it('rounds half-minor amounts only for display, retaining the exact fraction', () => {
    const value = usageAmount(3n, 2n)
    expect(formatUsageCost(value)).toBe(t('usageBreakdown.approximateAmount', { amount: '0.02 TXB' }))
    expect(value).toEqual({ numerator: 3n, denominator: 2n })
    expect(formatUsageCost(usageAmount(101n))).toBe('1.01 TXB')
  })
})
