import { describe, expect, it } from 'vitest'

import { chartSegments } from './statisticsFormat'

describe('statistics formatting', () => {
  it('drops invalid and non-positive chart facts', () => {
    const segments = chartSegments([
      { id: 'paid', label: 'Paid', value: 3 },
      { id: 'invalid', label: 'Invalid', value: Number.NaN },
      { id: 'empty', label: 'Empty', value: 0 },
    ])

    expect(segments).toHaveLength(1)
    expect(segments[0]?.percentage).toBe(100)
  })

  it('scales a series against a shared concentric-ring total', () => {
    const segments = chartSegments([{ id: 'ezpay', label: 'EZPay', value: 2 }], 20)

    expect(segments[0]?.percentage).toBe(20)
    expect(chartSegments([{ id: 'paid', label: 'Paid', value: 1 }], 0)[0]?.percentage).toBe(100)
  })
})
