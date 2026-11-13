import { describe, expect, it } from 'vitest'
import { draftFromDraw, serializeDraw } from './drawDraft'

function draft() {
  const result = draftFromDraw(null)
  result.name = 'Configured draw'
  result.prizes[0]!.name = 'Reward'
  result.prizes[0]!.kind = 'txb_delta'
  result.prizes[0]!.rangeMin = '1.00'
  result.prizes[0]!.rangeMax = '2.00'
  return result
}

describe('exact reward serialization', () => {
  it.each(['1.005', '10000000000.000000001', '1e2', '0x10', ''])('rejects an inexact or nondecimal range: %s', value => {
    const input = draft()
    input.prizes[0]!.rangeMin = value
    input.prizes[0]!.rangeMax = value
    expect(serializeDraw(input)).toBeNull()
  })

  it('preserves signed decimal ticks at the numeric API boundary', () => {
    const input = draft()
    input.prizes[0]!.rangeMin = '-1.29'
    input.prizes[0]!.rangeMax = '90071992547409.91'
    expect(serializeDraw(input)?.prizes[0]?.reward.range).toMatchObject({ min: -129, max: Number.MAX_SAFE_INTEGER })
  })

  it('rejects an entitlement renewal price that would lose minor units', () => {
    const input = draft()
    Object.assign(input.prizes[0]!, { kind: 'entitlement_grant', comboId: 'combo', renewalPrice: '90071992547409.93' })
    expect(serializeDraw(input)).toBeNull()
  })
})
