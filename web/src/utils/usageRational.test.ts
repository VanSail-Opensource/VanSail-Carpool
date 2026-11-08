import { describe, expect, it } from 'vitest'
import { addUsageAmounts, belowOnePercent, readUsageAmount, usageAmount, usageShare } from './usageRational'

describe('exact usage costs', () => {
  it('retains large amounts and fractional remainders instead of rounding node charges', () => {
    const result = addUsageAmounts(usageAmount(9007199254740993n, 2n), usageAmount(1n, 3n))
    expect(result).toEqual({ numerator: 27021597764222981n, denominator: 6n })
    expect(addUsageAmounts(usageAmount(1n, 3n), usageAmount(2n, 3n))).toEqual(usageAmount(1n))
    expect(usageAmount(0n, 999n)).toEqual({ numerator: 0n, denominator: 1n })
  })
  it('compares grouping thresholds exactly and converts only bounded drawing shares', () => {
    expect(belowOnePercent(usageAmount(1n), usageAmount(100n))).toBe(false)
    expect(belowOnePercent(usageAmount(1n), usageAmount(101n))).toBe(true)
    expect(usageShare(usageAmount(1n, 3n), usageAmount(1n))).toBeCloseTo(1 / 3, 11)
    expect(usageShare(usageAmount(0n), usageAmount(0n))).toBe(0)
    expect(usageShare(usageAmount(10n ** 1023n), usageAmount(1n))).toBe(1)
  })
  it.each([
    { numerator: '-1', denominator: '1' }, { numerator: '1', denominator: '0' },
    { numerator: 'NaN', denominator: '1' }, { numerator: '', denominator: '1' },
    { numerator: '1', denominator: '-3' }, { numerator: '1.5', denominator: '1' },
  ])('rejects invalid arithmetic inputs %j', value => { expect(() => readUsageAmount(value)).toThrow() })
})
