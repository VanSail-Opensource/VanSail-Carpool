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
  it('defaults to traffic and switches to recorded costs without mixing the measures', () => {
    const history = { from: '', to: '', hours: [], truncated: false, costAvailable: true, nodes: [
      { nodeId: 'a', name: 'A', countryCode: '', uploadBytes: '1', downloadBytes: '2', spentMinor: '1' },
      { nodeId: 'b', name: 'B', countryCode: '', uploadBytes: '1', downloadBytes: '0', spentMinor: '3' },
      { nodeId: 'credit', name: 'Credit only', countryCode: '', uploadBytes: '0', downloadBytes: '0', spentMinor: '4' },
    ] }
    expect(paygHistorySegments(history).segments.map(row => row.percentage)).toEqual([75, 25])
    expect(paygHistorySegments(history, 'cost').segments.map(row => row.percentage)).toEqual([12.5, 37.5, 50])
    expect(paygHistorySegments(history, 'cost').total).toBe('8')
  })
  it('preserves large cost strings and leaves missing costs empty', () => {
    const history = { from: '', to: '', hours: [], truncated: false, nodes: [
      { nodeId: 'a', name: 'A', countryCode: '', uploadBytes: '1', downloadBytes: '0', spentMinor: '9007199254740993' },
      { nodeId: 'b', name: 'B', countryCode: '', uploadBytes: '1', downloadBytes: '0' },
    ] }
    expect(paygHistorySegments(history, 'cost').total).toBe('9007199254740993')
    expect(paygHistorySegments(history, 'cost').segments).toHaveLength(1)
  })
})
