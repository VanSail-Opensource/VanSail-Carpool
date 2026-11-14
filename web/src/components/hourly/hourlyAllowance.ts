import type { HourlyView } from '@/api/hourly'

function validBytes(value: string | undefined): value is string {
  return value !== undefined && /^\d+$/.test(value)
}

function bytes(value: string | undefined): bigint {
  return validBytes(value) ? BigInt(value) : 0n
}

function minimum(value: bigint, ceiling: bigint): bigint {
  return value < ceiling ? value : ceiling
}

// Ordinary open allocations can still carry traffic. Only finalized recovery
// uncertainty is unavailable; it must never be relabelled as confirmed usage.
export function hourlyAllowance(usage: HourlyView['usage'], dailyAllowanceBytes: string) {
  const freeAllowance = bytes(usage?.freeAllowanceBytes ?? dailyAllowanceBytes)
  const extraFunded = bytes(usage?.extraFundedBytes)
  const freeUsed = minimum(bytes(usage?.freeUsedBytes), freeAllowance)
  const extraUsed = minimum(bytes(usage?.extraUsedBytes), extraFunded)
  const freeUnconsumed = freeAllowance - freeUsed
  const extraUnconsumed = extraFunded - extraUsed
  const accounting = usage?.accounting
  const splitKnown = validBytes(accounting?.freeRecoveryUnknownBytes) && validBytes(accounting?.extraRecoveryUnknownBytes)
  const freeUnknown = minimum(bytes(accounting?.freeRecoveryUnknownBytes), freeUnconsumed)
  const extraUnknown = minimum(bytes(accounting?.extraRecoveryUnknownBytes), extraUnconsumed)
  const unknown = splitKnown ? freeUnknown + extraUnknown : minimum(bytes(accounting?.recoveryUnknownBytes), freeUnconsumed + extraUnconsumed)
  const remaining = freeUnconsumed + extraUnconsumed - unknown
  let freeRemaining: bigint | null = splitKnown ? freeUnconsumed - freeUnknown : freeUnconsumed
  let extraRemaining: bigint | null = splitKnown ? extraUnconsumed - extraUnknown : extraUnconsumed
  // A cached older response may have only the aggregate. Keep its exact total
  // and exhaustion state without guessing which pool owns the recovery hold.
  if (!splitKnown && unknown > 0n) {
    freeRemaining = extraUnconsumed === 0n ? remaining : freeUnconsumed === 0n ? 0n : null
    extraRemaining = freeUnconsumed === 0n ? remaining : extraUnconsumed === 0n ? 0n : null
  }
  return {
    freeAllowance, extraFunded, freeUsed, extraUsed, freeRemaining, extraRemaining, unknown, remaining,
    total: freeAllowance + extraFunded,
    used: freeUsed + extraUsed,
    freeExhausted: freeRemaining === 0n || remaining === 0n,
    exhausted: remaining === 0n,
  }
}
