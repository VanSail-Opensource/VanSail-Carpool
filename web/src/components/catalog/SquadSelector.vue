<script setup lang="ts">
import { computed } from 'vue'

import type { SquadProduct } from '@/api/types'
import SquadPricingTable from './SquadPricingTable.vue'

type SquadType = NonNullable<SquadProduct['profile']>['type']

const props = defineProps<{
  squads: readonly SquadProduct[]
  selectedIds: readonly string[]
  includedIds: readonly string[]
  featuredIds: readonly string[]
  orderedIds: readonly string[]
  hiddenIds?: readonly string[]
}>()

const emit = defineEmits<{
  toggle: [id: string]
}>()

const types: readonly SquadType[] = ['international_network', 'broadband', 'china_optimized']
const squadOrder = computed(() => new Map(props.orderedIds.map((id, index) => [id, index])))
const visibleSquads = computed(() => props.squads
  .filter(squad => squad.profile && !props.includedIds.includes(squad.id) && !props.hiddenIds?.includes(squad.id))
  .map((squad, index) => ({ squad, index }))
  .sort((left, right) => {
    const fallback = props.orderedIds.length
    return (squadOrder.value.get(left.squad.id) ?? fallback + left.index)
      - (squadOrder.value.get(right.squad.id) ?? fallback + right.index)
  })
  .map(row => row.squad))

function squadsFor(type: SquadType): readonly SquadProduct[] {
  return visibleSquads.value.filter(squad => squad.profile?.type === type)
}
</script>

<template>
  <section class="squad-selector">
    <div class="section-heading section-heading--stacked">
      <h2>{{ $t('catalog.optionalSquads') }}</h2>
    </div>
    <div v-if="visibleSquads.length" class="squad-pricing-groups">
      <template v-for="type in types" :key="type">
        <SquadPricingTable
          v-if="squadsFor(type).length"
          :profile-type="type"
          :squads="squadsFor(type)"
          :selected-ids="selectedIds"
          :included-ids="includedIds"
          :featured-ids="featuredIds"
          @toggle="emit('toggle', $event)"
        />
      </template>
    </div>
    <div v-else class="empty-inline">
      <div>
        <h3>{{ $t('catalog.noSquads') }}</h3>
        <p>{{ $t('catalog.noSquadsHint') }}</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.squad-pricing-groups { display: grid; gap: 1.25rem; }
@media (min-width: 900px) {
  .squad-pricing-groups { gap: 0.85rem; }
}
</style>
