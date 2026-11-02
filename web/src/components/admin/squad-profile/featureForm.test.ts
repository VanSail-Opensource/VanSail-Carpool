import { describe, expect, it } from 'vitest'

import { normalizedSquadFeatures, squadFeaturesPatch, validSquadFeatures, type SquadFeature } from './featureForm'

describe('squad feature form', () => {
  it('validates Unicode character limits and the allowed icons', () => {
    const feature: SquadFeature = { icon: 'route', title: '航'.repeat(80), detail: '😀'.repeat(160) }
    expect(validSquadFeatures([feature])).toBe(true)
    expect(validSquadFeatures([{ ...feature, title: '航'.repeat(81) }])).toBe(false)
    expect(validSquadFeatures([{ ...feature, detail: '😀'.repeat(161) }])).toBe(false)
    expect(validSquadFeatures([{ ...feature, title: '   ' }])).toBe(false)
    expect(validSquadFeatures([{ ...feature, icon: 'unknown' as SquadFeature['icon'] }])).toBe(false)
    expect(validSquadFeatures(Array.from({ length: 13 }, () => feature))).toBe(false)
  })

  it('preserves order, trims text, and omits an empty optional detail', () => {
    expect(normalizedSquadFeatures([
      { icon: 'route', title: ' Transit ', detail: ' Regional ' },
      { icon: 'shield', title: ' Private ', detail: '   ' },
    ])).toEqual([{ icon: 'route', title: 'Transit', detail: 'Regional' }, { icon: 'shield', title: 'Private' }])
    expect(validSquadFeatures([])).toBe(true)
  })

  it('omits untouched features and sends an explicit empty list when cleared', () => {
    const draft: SquadFeature[] = [{ icon: 'route', title: 'Old value' }]
    expect(squadFeaturesPatch(draft, false)).toEqual({})
    expect(squadFeaturesPatch([], false)).toEqual({})
    expect(squadFeaturesPatch([], true)).toEqual({ features: [] })
    expect(squadFeaturesPatch(draft, true)).toEqual({ features: draft })
  })
})
