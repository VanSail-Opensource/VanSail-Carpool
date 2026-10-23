import { describe, expect, it } from 'vitest'

import type { NormalizedDistribution, SquadProduct } from '@/api/types'
import { catalogSquadPresentation } from './catalogSquadPresentation'

function squad(id: string, groupId: string, stockRemaining: number | null = null): SquadProduct {
  return {
    id,
    groupId,
    name: id,
    description: '',
    profile: null,
    price: { currency: 'TXB', minor: '100', display: '1.00 TXB' },
    visible: true,
  upstreamPresent: true,
  stockHeldByCurrentUser: false,
  activationRequired: false,
    accessibleNodes: [],
    stockRemaining,
    createdAt: '2026-08-21T00:00:00Z',
    updatedAt: '2026-08-21T00:00:00Z',
  }
}

const squads = [
  squad('included', '00000000-0000-4000-8000-000000000001'),
  squad('full', '00000000-0000-4000-8000-000000000002', 0),
  squad('featured-a', '00000000-0000-4000-8000-000000000003'),
  squad('featured-b', '00000000-0000-4000-8000-000000000004'),
  squad('other', '00000000-0000-4000-8000-000000000005'),
]

const distributions: readonly NormalizedDistribution[] = [{
  id: 'combo-1',
  label: 'Core',
  segments: [
    { id: squads[0]!.groupId, label: 'Included', value: 90 },
    { id: squads[1]!.groupId, label: 'Full', value: 80 },
    { id: squads[2]!.groupId, label: 'Featured A', value: 40 },
    { id: squads[3]!.groupId, label: 'Featured B', value: 40 },
    { id: squads[4]!.groupId, label: 'Other', value: 10 },
  ],
}]

describe('catalogSquadPresentation', () => {
  it('sorts selectable squads by composition and moves included and full squads to the bottom', () => {
    expect(catalogSquadPresentation(squads, ['included'], distributions, 'combo-1'))
      .toEqual({
        featuredIds: ['featured-a', 'featured-b'],
        orderedIds: ['other', 'featured-a', 'featured-b', 'included', 'full'],
      })
  })

  it('keeps selectable and bottom groups stable without matching composition data', () => {
    expect(catalogSquadPresentation(squads, ['included'], distributions, 'missing'))
      .toEqual({
        featuredIds: [],
        orderedIds: ['featured-a', 'featured-b', 'other', 'included', 'full'],
      })
  })

  it('uses global composition and calculates featured leaders per squad type before combo selection', () => {
    const typed = [
      { ...squads[2]!, profile: { type: 'broadband' as const, isp: 'A', portMbps: 100, dynamic: false, location: 'A' } },
      { ...squads[3]!, profile: { type: 'broadband' as const, isp: 'B', portMbps: 100, dynamic: false, location: 'B' } },
      { ...squads[4]!, profile: { type: 'international_network' as const, portMbps: null, countryCode: 'SG', upstreamCarriers: ['C'] } },
    ]
    const global = [
      distributions[0]!,
      { ...distributions[0]!, id: 'combo-2', segments: [
        { id: typed[0]!.groupId, label: 'A', value: 10 },
        { id: typed[1]!.groupId, label: 'B', value: 30 },
        { id: typed[2]!.groupId, label: 'C', value: 5 },
      ] },
    ]

    expect(catalogSquadPresentation(typed, [], global, null)).toEqual({
      featuredIds: [typed[1]!.id, typed[2]!.id],
      orderedIds: [typed[0]!.id, typed[1]!.id, typed[2]!.id],
    })
  })
})
