<script setup lang="ts">
import { LayoutGroup } from 'motion-v'
import type { Combo } from '@/api/types'
import ComboOption from './ComboOption.vue'

const props = defineProps<{
  combos: readonly Combo[]
  selectedId: string | null
}>()

const emit = defineEmits<{ select: [id: string] }>()
</script>

<template>
  <div class="combo-pricing">
    <LayoutGroup id="catalog-combo-selection">
      <div class="combo-grid">
        <ComboOption
          v-for="combo in props.combos"
          :key="combo.id"
          :combo="combo"
          :selected="combo.id === props.selectedId"
          @select="emit('select', $event)"
        />
      </div>
    </LayoutGroup>
  </div>
</template>

<style scoped>
.combo-grid {
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 20rem), 1fr));
  align-items: start;
}
@media (min-width: 900px) {
  .combo-grid { gap: 0.65rem; }
}
</style>
