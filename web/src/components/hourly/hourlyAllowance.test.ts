import { describe, expect, it } from 'vitest'
import { hourlyFixture } from '../../../fixtures/hourly-data'
import { hourlyAllowance } from './hourlyAllowance'

describe('usable hourly allowance', () => {
  it('does not offer the 109.81 MiB recovery residual or report it as consumed', () => {
    const { view } = hourlyFixture(new URLSearchParams('recovery-exhausted'))
    const allowance = hourlyAllowance(view.usage, view.policy.policy.dailyAllowanceBytes)
    expect(allowance.remaining).toBe(0n)
    expect(allowance.used).toBe(2032331516n)
    expect(allowance.unknown).toBe(115152132n)
    expect(allowance.exhausted).toBe(true)
  })

  it('retains the same residual when it is an ordinary open allocation', () => {
    const { view } = hourlyFixture(new URLSearchParams('open-credit'))
    const allowance = hourlyAllowance(view.usage, view.policy.policy.dailyAllowanceBytes)
    expect(allowance.remaining).toBe(115152132n)
    expect(allowance.used).toBe(2032331516n)
    expect(allowance.unknown).toBe(0n)
    expect(allowance.exhausted).toBe(false)
  })

  it('separates free and purchased recovery without consuming either open allocation', () => {
    const { view } = hourlyFixture(new URLSearchParams())
    const usage = {
      ...view.usage!, freeAllowanceBytes: '1024', freeUsedBytes: '512', freeReservedBytes: '512',
      extraFundedBytes: '2048', extraUsedBytes: '1024', extraReservedBytes: '1024',
      accounting: { ...view.usage!.accounting!, recoveryUnknownBytes: '768', freeRecoveryUnknownBytes: '512', extraRecoveryUnknownBytes: '256' },
    }
    const allowance = hourlyAllowance(usage, '1024')
    expect(allowance.freeRemaining).toBe(0n)
    expect(allowance.extraRemaining).toBe(768n)
    expect(allowance.freeExhausted).toBe(true)
    expect(allowance.exhausted).toBe(false)
    expect(allowance.used).toBe(1536n)
  })

  it('does not deduct historical recovery from the current UTC-day allowance', () => {
    const { view } = hourlyFixture(new URLSearchParams('first&historical-unknown&historical-unsealed'))
    expect(view.usage!.accountingPending).toBe(true)
    const allowance = hourlyAllowance(view.usage, view.policy.policy.dailyAllowanceBytes)
    expect(allowance.remaining).toBe(10000000000n)
    expect(allowance.exhausted).toBe(false)
  })

  it('keeps byte accuracy above the JavaScript safe-integer range', () => {
    const { view } = hourlyFixture(new URLSearchParams('first'))
    const usage = { ...view.usage!, freeAllowanceBytes: '9007199254740993', freeUsedBytes: '9007199254740991' }
    const allowance = hourlyAllowance(usage, '0')
    expect(allowance.remaining).toBe(2n)
    expect(allowance.used).toBe(9007199254740991n)
  })

  it('keeps an older aggregate-only snapshot exact without inventing a pool split', () => {
    const { view } = hourlyFixture(new URLSearchParams('current-unknown'))
    const usage = structuredClone(view.usage!)
    Reflect.deleteProperty(usage.accounting!, 'freeRecoveryUnknownBytes')
    Reflect.deleteProperty(usage.accounting!, 'extraRecoveryUnknownBytes')
    const allowance = hourlyAllowance(usage, view.policy.policy.dailyAllowanceBytes)
    expect(allowance.remaining).toBe(8499868928n)
    expect(allowance.freeRemaining).toBeNull()
    expect(allowance.extraRemaining).toBeNull()
    expect(allowance.used).toBe(3500000000n)
    expect(allowance.exhausted).toBe(false)
  })

  it('recognizes exhausted recovery even in an older aggregate-only snapshot', () => {
    const { view } = hourlyFixture(new URLSearchParams('recovery-exhausted'))
    const usage = structuredClone(view.usage!)
    Reflect.deleteProperty(usage.accounting!, 'freeRecoveryUnknownBytes')
    Reflect.deleteProperty(usage.accounting!, 'extraRecoveryUnknownBytes')
    expect(hourlyAllowance(usage, view.policy.policy.dailyAllowanceBytes).exhausted).toBe(true)
  })
})
