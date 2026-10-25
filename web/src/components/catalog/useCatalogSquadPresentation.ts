import { computed, type ComputedRef, type Ref } from 'vue'

import type { SquadProduct } from '@/api/types'
import { catalogSquadPresentation } from './catalogSquadPresentation'

export function useCatalogSquadPresentation(
  squads: ComputedRef<readonly SquadProduct[]>,
  includedIds: ComputedRef<readonly string[]>,
  comboId: Ref<string | null>,
) {
  // Statistics belong to the browser console. Member ordering uses catalog
  // order, profile and stock without attempting a forbidden admin read.
  const presentation = computed(() => catalogSquadPresentation(
    squads.value,
    includedIds.value,
    [],
    comboId.value,
  ))

  return {
    featuredIds: computed(() => presentation.value.featuredIds),
    orderedIds: computed(() => presentation.value.orderedIds),
  }
}
