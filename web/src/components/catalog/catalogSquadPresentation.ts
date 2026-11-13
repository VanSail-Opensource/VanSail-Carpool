import type { SquadProduct } from '@/api/types'

export interface CatalogSquadPresentation {
  orderedIds: readonly string[]
}

export function catalogSquadPresentation(
  squads: readonly SquadProduct[],
  includedIds: readonly string[],
): CatalogSquadPresentation {
  const included = new Set(includedIds)
  const rows = squads.map((squad) => ({
    id: squad.id,
    bottom: included.has(squad.id) || (squad.stockRemaining === 0 && !squad.stockHeldByCurrentUser),
    type: squad.profile?.type ?? 'unconfigured',
  }))
  const types = [...new Set(rows.map(row => row.type))]
  const ordered = types.flatMap((type) => {
    const group = rows.filter(row => row.type === type)
    const selectable = group.filter(row => !row.bottom)
    return [...selectable, ...group.filter(row => row.bottom)]
  })
  return {
    orderedIds: ordered.map(row => row.id),
  }
}
