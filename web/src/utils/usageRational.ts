import type { RationalMinor } from '@/api/usageBreakdown'

export interface ExactUsageAmount { numerator: bigint; denominator: bigint }
export const zeroUsageAmount = (): ExactUsageAmount => ({ numerator: 0n, denominator: 1n })

function gcd(a: bigint, b: bigint): bigint {
  while (b !== 0n) [a, b] = [b, a % b]
  return a
}

export function usageAmount(numerator: bigint, denominator = 1n): ExactUsageAmount {
  if (numerator < 0n || denominator <= 0n) throw new Error('USAGE_AMOUNT_INVALID')
  const factor = gcd(numerator, denominator)
  return { numerator: numerator / factor, denominator: denominator / factor }
}

export function readUsageAmount(value: RationalMinor): ExactUsageAmount {
  if (!/^[0-9]{1,1024}$/.test(value.numerator) || !/^[1-9][0-9]{0,1023}$/.test(value.denominator)) throw new Error('USAGE_AMOUNT_INVALID')
  return usageAmount(BigInt(value.numerator), BigInt(value.denominator))
}

export function addUsageAmounts(a: ExactUsageAmount, b: ExactUsageAmount): ExactUsageAmount {
  const factor = gcd(a.denominator, b.denominator)
  return usageAmount(a.numerator * (b.denominator / factor) + b.numerator * (a.denominator / factor), a.denominator * (b.denominator / factor))
}

export function usageShare(value: ExactUsageAmount, total: ExactUsageAmount): number {
  if (total.numerator <= 0n || value.numerator <= 0n) return 0
  if (value.denominator <= 0n || total.denominator <= 0n) throw new Error('USAGE_AMOUNT_INVALID')
  const numerator = value.numerator * total.denominator
  const denominator = value.denominator * total.numerator
  if (numerator >= denominator) return 1
  // Only bounded drawing proportions become floating point. Financial amounts
  // and grouping comparisons retain their complete integer/rational values.
  return Number(numerator * 1_000_000_000_000n / denominator) / 1_000_000_000_000
}

export function belowOnePercent(value: ExactUsageAmount, total: ExactUsageAmount): boolean {
  return value.numerator * total.denominator * 100n < total.numerator * value.denominator
}
