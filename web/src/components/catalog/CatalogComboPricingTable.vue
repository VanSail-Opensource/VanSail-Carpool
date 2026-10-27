<script setup lang="ts">
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
    <div class="combo-grid">
      <ComboOption
        v-for="combo in props.combos"
        :key="combo.id"
        :combo="combo"
        :selected="combo.id === props.selectedId"
        @select="emit('select', $event)"
      />
    </div>
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
