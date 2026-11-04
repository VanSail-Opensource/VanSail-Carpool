import { describe, expect, it } from 'vitest'
import { paygHistorySegments } from './paygHistorySegments'

describe('PAYG history shares', () => {
  it('preserves exact totals beyond browser integer precision and only draws bounded shares', () => {
    const value = paygHistorySegments({ from: '', to: '', hours: [], truncated: true, nodes: [
      { nodeId: 'retired', name: 'Historical node', countryCode: '', uploadBytes: '9007199254740993', downloadBytes: '1024' },
      { nodeId: 'active', name: 'Current node', countryCode: '', uploadBytes: '1', downloadBytes: '0' },
    ] })
    expect(value.total).toBe('9007199254742018')
    expect(value.segments[0]?.bytes).toBe('9007199254742017')
    expect(value.segments.every(segment => segment.share >= 0 && segment.share <= 1)).toBe(true)
    expect(value.segments).toHaveLength(2)
  })
  it('keeps a zero-use window empty', () => {
    expect(paygHistorySegments({ from: '', to: '', hours: [], nodes: [], truncated: false })).toEqual({ total: '0', segments: [] })
  })
})
