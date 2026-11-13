import { computed, type ComputedRef } from 'vue'

import type { SquadProduct } from '@/api/types'
import { catalogSquadPresentation } from './catalogSquadPresentation'

export function useCatalogSquadPresentation(
  squads: ComputedRef<readonly SquadProduct[]>,
  includedIds: ComputedRef<readonly string[]>,
) {
  // Statistics belong to the browser console. Member ordering uses catalog
  // order, profile and stock without attempting a forbidden admin read.
  const presentation = computed(() => catalogSquadPresentation(
    squads.value,
    includedIds.value,
  ))

  return {
    orderedIds: computed(() => presentation.value.orderedIds),
  }
}
