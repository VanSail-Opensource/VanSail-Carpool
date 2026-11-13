import { describe, expect, it } from 'vitest'

import type { SquadProduct } from '@/api/types'
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
  squad('available-a', '00000000-0000-4000-8000-000000000003'),
  squad('available-b', '00000000-0000-4000-8000-000000000004'),
  squad('other', '00000000-0000-4000-8000-000000000005'),
]

describe('catalogSquadPresentation', () => {
  it('preserves catalog order and moves included and full squads to the bottom', () => {
    expect(catalogSquadPresentation(squads, ['included']))
      .toEqual({
        orderedIds: ['available-a', 'available-b', 'other', 'included', 'full'],
      })
  })

  it('keeps stock held by the current member selectable', () => {
    expect(catalogSquadPresentation(squads.map(item => item.id === 'full' ? { ...item, stockHeldByCurrentUser: true } : item), ['included']))
      .toEqual({
        orderedIds: ['full', 'available-a', 'available-b', 'other', 'included'],
      })
  })

  it('preserves catalog order within each profile type', () => {
    const typed = [
      { ...squads[2]!, profile: { type: 'broadband' as const, isp: 'A', portMbps: 100, dynamic: false, location: 'A' } },
      { ...squads[3]!, profile: { type: 'broadband' as const, isp: 'B', portMbps: 100, dynamic: false, location: 'B' } },
      { ...squads[4]!, profile: { type: 'international_network' as const, portMbps: null, countryCode: 'SG', upstreamCarriers: ['C'] } },
    ]
    expect(catalogSquadPresentation([typed[0]!, typed[2]!, typed[1]!], [])).toEqual({
      orderedIds: [typed[0]!.id, typed[1]!.id, typed[2]!.id],
    })
  })
})
