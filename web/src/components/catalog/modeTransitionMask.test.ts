import { describe, expect, it } from 'vitest'
import { modeTransitionMask } from './modeTransitionMask'

describe('uncovered transition controls', () => {
  it('merges overlapping controls into one cutout, including their motion reserve', () => {
    const path = modeTransitionMask(320, 800, [
      { left: 12, top: 12, right: 150, bottom: 56 },
      { left: 154, top: 12, right: 308, bottom: 56 },
      { left: 250, top: 110, right: 308, bottom: 154 },
    ])
    expect(path.match(/M/g)).toHaveLength(3)
    expect(path).toContain('M4 4H316V64H4Z')
    expect(path).toContain('M242 102H316V162H242Z')
  })

  it('ignores controls outside the viewport and clips partial controls to its edge', () => {
    const path = modeTransitionMask(320, 800, [
      { left: 0, top: 900, right: 100, bottom: 950 },
      { left: -10, top: 780, right: 100, bottom: 850 },
    ])
    expect(path.match(/M/g)).toHaveLength(2)
    expect(path).toContain('M0 772H108V800H0Z')
  })
})
