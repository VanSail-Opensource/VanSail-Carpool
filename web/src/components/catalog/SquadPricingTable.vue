<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { SquadProduct } from '@/api/types'
import SquadPricingCard from './SquadPricingCard.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'

type SquadType = NonNullable<SquadProduct['profile']>['type']

const props = defineProps<{
  profileType: SquadType
  squads: readonly SquadProduct[]
  selectedIds: readonly string[]
  includedIds: readonly string[]
  featuredIds: readonly string[]
}>()

const emit = defineEmits<{
  toggle: [id: string]
}>()

const typeKey = computed(() => props.profileType === 'china_optimized'
  ? 'chinaOptimized'
  : props.profileType === 'international_network' ? 'internationalNetwork' : 'broadband')
const { reducedMotion } = useMotionPreferences()
</script>

<template>
  <section class="squad-pricing-table" :class="`squad-pricing-table--${profileType}`">
    <h3>{{ $t(`squadProfile.types.${typeKey}`) }}</h3>
    <motion.div layout class="squad-pricing-grid">
      <AnimatePresence :initial="false" mode="popLayout">
        <motion.div v-for="squad in props.squads" :key="squad.id" layout :initial="reducedMotion ? false : { opacity: 0, y: 6 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
          <SquadPricingCard
            :squad="squad"
            :selected="props.selectedIds.includes(squad.id)"
            :included="props.includedIds.includes(squad.id)"
            :featured="props.featuredIds.includes(squad.id)"
            @toggle="emit('toggle', $event)"
          />
        </motion.div>
      </AnimatePresence>
    </motion.div>
  </section>
</template>

<style scoped>
.squad-pricing-table { min-width: 0; display: grid; gap: 0.55rem; }
.squad-pricing-table > h3 { margin: 0; font-size: 0.92rem; }
.squad-pricing-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr));
  align-items: start;
  gap: 0.65rem;
}
@media (min-width: 900px) {
  .squad-pricing-grid {
    display: block;
    column-width: 21rem;
    column-gap: 0.65rem;
  }
  .squad-pricing-grid > * {
    break-inside: avoid;
    margin-bottom: 0.65rem;
  }
}
</style>
