<script setup lang="ts">
import LivePrice from '@/components/common/LivePrice.vue'
import { computed } from 'vue'

import type { SquadProduct } from '@/api/types'
import SquadProfileFacts from '@/components/squad-profile/SquadProfileFacts.vue'
import SquadProfileSummary from '@/components/squad-profile/SquadProfileSummary.vue'
import { formatMemberMoney } from '@/utils/displayCurrency'
import { selectionHaptic } from '@/utils/telegram'

const props = defineProps<{
  informational?: boolean
  squad: SquadProduct
  selected: boolean
  included: boolean
  featured: boolean
}>()

const emit = defineEmits<{
  toggle: [id: string]
}>()
const isFull = computed(() => !props.included && props.squad.stockRemaining === 0 && !props.squad.stockHeldByCurrentUser)
const remainingPercentage = computed(() => {
  if (props.squad.stockLimit === null || props.squad.stockLimit === undefined) return null
  if (props.squad.stockLimit <= 0) return 0
  const count = Math.max(0, Math.min(props.squad.stockLimit, props.squad.stockRemaining ?? props.squad.stockLimit))
  return count * 100 / props.squad.stockLimit
})
const remainingBadge = computed(() => {
  const percentage = remainingPercentage.value
  const unlimited = percentage === null
  const scarce = percentage !== null && percentage <= 20
  const abundant = percentage !== null && percentage >= 80
  return {
    color: unlimited || abundant ? 'neutral' : scarce ? 'warning' : 'info',
    icon: unlimited ? 'i-lucide-infinity' : abundant ? 'i-lucide-user-plus' : scarce ? 'i-lucide-users' : 'i-lucide-user',
    label: percentage === null ? '∞' : `${Math.round(percentage)}%`,
  } as const
})

function toggle(): void {
  if (props.included || isFull.value) return
  selectionHaptic()
  emit('toggle', props.squad.id)
}
</script>

<template>
  <article
    class="squad-pricing-card"
    :class="[
      `squad-pricing-card--${squad.profile?.type ?? 'unconfigured'}`,
      {
        'squad-pricing-card--selected': selected,
        'squad-pricing-card--included': included,
        'squad-pricing-card--nonselectable': informational || included || isFull,
      },
    ]"
    :aria-label="squad.name"
  >
    <UButton
      v-if="!informational && !included && !isFull"
      type="button"
      color="neutral"
      variant="ghost"
      class="squad-card__select"
      :aria-label="squad.name"
      :aria-pressed="selected"
      @click="toggle"
    />
    <div class="squad-card__header">
      <SquadProfileSummary
        :name="squad.name"
        :profile="squad.profile"
        :description="squad.visible ? squad.description : undefined"
        compact
        presentation="member"
        :show-facts="false"
      >
        <template #namePrefix>
          <UIcon v-if="featured" class="squad-card__featured" name="i-ph-crown" :aria-label="$t('catalog.featured')" />
        </template>
        <template v-if="!informational || $slots.status" #nameTags>
          <slot name="status" />
          <UIcon v-if="!informational && !squad.visible" name="i-ph-lock-key" :aria-label="$t('catalog.hidden')" />
          <UBadge v-if="!informational && squad.activationRequired" size="sm" class="shrink-0 rounded-full px-1.5 py-0.5 font-bold" color="warning" variant="solid" icon="i-ph-lock-key" :aria-label="$t('catalog.activationRequired')" />
          <UBadge v-if="!informational && isFull" size="sm" class="shrink-0 rounded-full px-1.5 py-0.5 font-bold" color="error" variant="solid" icon="i-lucide-user-x" :aria-label="$t('catalog.full')" />
          <UBadge v-else-if="!informational && !included" size="sm" class="shrink-0 rounded-full px-1.5 py-0.5 font-bold" :color="remainingBadge.color" variant="solid" :icon="remainingBadge.icon" :aria-label="`${$t('catalog.remaining')} ${remainingBadge.label}`" />
        </template>
      </SquadProfileSummary>
      <div v-if="!included" class="squad-card__price">
        <slot name="price"><strong>{{ formatMemberMoney(squad.price) }}</strong><LivePrice :decision="squad.pricingDecision" /></slot>
      </div>
    </div>

    <SquadProfileFacts :profile="squad.profile" presentation="member" />
    <slot />
  </article>
</template>

<style scoped>
.squad-pricing-card {
  position: relative;
  min-width: 0;
  display: grid;
  gap: 0.45rem;
  padding: 0.6rem;
  border: 1px solid var(--line);
  border-radius: var(--radius-control);
  background: color-mix(in srgb, var(--surface-raised) 78%, transparent);
  cursor: pointer;
}
.squad-card__select { position: absolute; z-index: 1; inset: 0; width: 100%; min-height: 44px; padding: 0; border: 0; border-radius: inherit; background: transparent; cursor: pointer; }
.squad-card__select:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.squad-pricing-card--nonselectable { cursor: default; }
.squad-pricing-card--broadband { --squad-profile-tone: var(--accent); }
.squad-pricing-card--china_optimized { --squad-profile-tone: var(--warning); }
.squad-pricing-card--international_network { --squad-profile-tone: #9ebddd; }
.squad-pricing-card--selected {
  border-color: var(--squad-profile-tone, var(--accent));
  background: color-mix(in srgb, var(--accent) 9%, var(--surface-raised));
}
.squad-pricing-card--included { opacity: 0.78; }
.squad-card__header { min-width: 0; display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 0.65rem; align-items: start; }
.squad-card__price { min-width: 0; padding-top: 0.05rem; text-align: right; }
.squad-card__price > strong { display: block; white-space: nowrap; font-family: var(--font-mono); font-size: 1.1rem; }
@media (min-width: 900px) {
  .squad-pricing-card { gap: 0.4rem; padding: 0.55rem; }
  .squad-card__header { gap: 0.45rem; }
}
</style>
