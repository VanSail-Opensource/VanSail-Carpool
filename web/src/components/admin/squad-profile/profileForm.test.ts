import { describe, expect, it } from 'vitest'

import { draftFromProfile, profileFromDraft } from './profileForm'

describe('squad profile form mapping', () => {
  it('maps unlimited international ports and comma-separated carriers', () => {
    const draft = draftFromProfile(null)
    draft.countryCode = 'TW'
    draft.upstreamCarriers = 'Carrier A, Carrier B, Carrier A'
    const profile = profileFromDraft(draft)
    expect(profile).toEqual({
      type: 'international_network',
      portMbps: null,
      countryCode: 'TW',
      upstreamCarriers: ['Carrier A', 'Carrier B'],
    })
  })

  it('does not create an incomplete broadband profile', () => {
    const draft = draftFromProfile(null)
    draft.type = 'broadband'
    expect(profileFromDraft(draft)).toBeNull()
  })

  it('requires an explicit finite port when switching an unlimited profile to broadband', () => {
    const draft = draftFromProfile(null)
    Object.assign(draft, { type: 'broadband', isp: 'Example ISP', location: 'Example city' })
    expect(profileFromDraft(draft)).toBeNull()
    draft.portMbps = 1000
    expect(profileFromDraft(draft)).toMatchObject({ type: 'broadband', portMbps: 1000 })
    draft.portMbps = Number.MAX_SAFE_INTEGER + 1
    expect(profileFromDraft(draft)).toBeNull()
  })

  it('normalizes ISO alpha-2 country codes and rejects unknown codes', () => {
    const draft = draftFromProfile(null)
    draft.countryCode = 'sg'
    draft.upstreamCarriers = 'Carrier A'
    expect(profileFromDraft(draft)).toMatchObject({ countryCode: 'SG' })

    draft.countryCode = 'ZZ'
    expect(profileFromDraft(draft)).toBeNull()
  })
})
