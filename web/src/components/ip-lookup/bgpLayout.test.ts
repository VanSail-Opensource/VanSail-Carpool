import { describe, expect, it } from 'vitest'
import { mergeSegments } from './bgpLayout'

describe('route trunk merging', () => {
  it('preserves gaps and unions only coincident observed geometry', () => {
    const paths = mergeSegments([[10, 0, 10, 20], [10, 30, 10, 50], [10, 15, 10, 5], [12, 0, 12, 20], [0, 4, 20, 4], [10, 4, 30, 4]])
    expect(paths).toContain('M 10 0 L 10 20')
    expect(paths).toContain('M 10 30 L 10 50')
    expect(paths).not.toContain('M 10 0 L 10 50')
    expect(paths).toContain('M 12 0 L 12 20')
    expect(paths).toContain('M 0 4 L 30 4')
  })
})
