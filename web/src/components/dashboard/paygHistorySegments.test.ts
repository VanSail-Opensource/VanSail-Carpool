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
  it('combines only shares below one percent, independently for each measure', () => {
    const history = { from: '', to: '', hours: [], truncated: false, costAvailable: true, nodes: [
      { nodeId: 'major', name: 'Major', countryCode: '', uploadBytes: '9900', downloadBytes: '0', spentMinor: '1' },
      { nodeId: 'exact', name: 'Exact', countryCode: '', uploadBytes: '100', downloadBytes: '0', spentMinor: '99' },
      { nodeId: 'small', name: 'Small', countryCode: '', uploadBytes: '99', downloadBytes: '0', spentMinor: '9900' },
      { nodeId: 'tiny', name: 'Tiny', countryCode: '', uploadBytes: '1', downloadBytes: '0', spentMinor: '0' },
    ] }
    const traffic = paygHistorySegments(history)
    expect(traffic.segments.map(segment => segment.nodeId)).toEqual(['major', 'exact', '__payg_others__'])
    expect(traffic.segments[2]).toMatchObject({ bytes: '100', percentage: 1, isOther: true })

    const cost = paygHistorySegments(history, 'cost')
    expect(cost.segments.map(segment => segment.nodeId)).toEqual(['small', '__payg_others__'])
    expect(cost.segments[1]).toMatchObject({ bytes: '100', percentage: 1, isOther: true })
    expect(traffic.segments.reduce((sum, segment) => sum + BigInt(segment.bytes), 0n)).toBe(BigInt(traffic.total))
    expect(cost.segments.reduce((sum, segment) => sum + BigInt(segment.bytes), 0n)).toBe(BigInt(cost.total))
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
