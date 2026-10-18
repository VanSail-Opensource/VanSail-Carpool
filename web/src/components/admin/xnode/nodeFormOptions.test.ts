import { describe, expect, it } from 'vitest'
import { nodeMultiplierSchema } from './nodeFormOptions'

describe('explicit multiplier input', () => {
  it('preserves zero and six decimal places through the maximum', () => {
    for (const value of ['0', '0.000001', '1.123456', '1000', '1000.000000']) expect(nodeMultiplierSchema().safeParse(value).success).toBe(true)
  })
  it('rejects missing/null, excess fractional precision, exponent notation and out-of-range input', () => {
    for (const value of [undefined, null, '', '-0.000001', '0.0000001', '1.0000000', '1000.000001', '1e-7']) expect(nodeMultiplierSchema().safeParse(value).success).toBe(false)
  })
})
