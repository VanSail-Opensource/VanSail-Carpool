import type { NormalizedDistribution, SquadProduct } from '@/api/types'

export interface CatalogSquadPresentation {
  featuredIds: readonly string[]
  orderedIds: readonly string[]
}

export function catalogSquadPresentation(
  squads: readonly SquadProduct[],
  includedIds: readonly string[],
  distributions: readonly NormalizedDistribution[],
  comboId: string | null,
): CatalogSquadPresentation {
  const composition = comboId ? distributions.find(row => row.id === comboId) : undefined
  const shares = comboId === null
    ? globalShares(distributions)
    : new Map(composition?.segments.map(segment => [segment.id, segment.value]) ?? [])
  const included = new Set(includedIds)
  const rows = squads.map((squad, index) => {
    const value = shares.get(squad.groupId) ?? 0
    return {
      id: squad.id,
      index,
      bottom: included.has(squad.id) || (squad.stockRemaining === 0 && !squad.stockHeldByCurrentUser),
      share: Number.isFinite(value) && value > 0 ? value : 0,
      type: squad.profile?.type ?? 'unconfigured',
    }
  })
  const types = [...new Set(rows.map(row => row.type))]
  const ordered = types.flatMap((type) => {
    const group = rows.filter(row => row.type === type)
    const selectable = group.filter(row => !row.bottom).sort((left, right) => left.share - right.share || left.index - right.index)
    return [...selectable, ...group.filter(row => row.bottom)]
  })
  const featuredIds = types.flatMap((type) => {
    const selectable = rows.filter(row => row.type === type && !row.bottom)
    const maximum = Math.max(0, ...selectable.map(row => row.share))
    return maximum > 0 ? selectable.filter(row => row.share === maximum).map(row => row.id) : []
  })

  return {
    featuredIds,
    orderedIds: ordered.map(row => row.id),
  }
}

function globalShares(distributions: readonly NormalizedDistribution[]): Map<string, number> {
  const shares = new Map<string, number>()
  for (const distribution of distributions) {
    for (const segment of distribution.segments) {
      if (!Number.isFinite(segment.value) || segment.value <= 0) continue
      shares.set(segment.id, (shares.get(segment.id) ?? 0) + segment.value)
    }
  }
  return shares
}
