<script setup lang="ts">
import type { SquadProduct } from '@/api/types'
import SquadSelector from './SquadSelector.vue'

defineProps<{
  squads: readonly SquadProduct[]
  selectedIds: readonly string[]
  includedIds: readonly string[]
  orderedIds: readonly string[]
  hiddenIds?: readonly string[]
}>()

const emit = defineEmits<{ toggle: [id: string] }>()
</script>

<template>
  <SquadSelector
    :squads="squads"
    :selected-ids="selectedIds"
    :included-ids="includedIds"
    :ordered-ids="orderedIds"
    :hidden-ids="hiddenIds"
    @toggle="emit('toggle', $event)"
  />
</template>
