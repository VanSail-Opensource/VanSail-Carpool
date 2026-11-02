<script setup lang="ts">
import { computed } from 'vue'

import type { SquadProduct } from '@/api/types'
import SquadProfileFacts from '@/components/squad-profile/SquadProfileFacts.vue'
import SquadProfileSummary from '@/components/squad-profile/SquadProfileSummary.vue'
import { selectionHaptic } from '@/utils/telegram'
import SquadFeatures from './SquadFeatures.vue'

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
    icon: unlimited ? 'i-ph-infinity' : abundant ? 'i-ph-user-plus' : scarce ? 'i-ph-users' : 'i-ph-user',
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
      :aria-label="$t('catalog.selectSquadNamed', { name: squad.name })"
      :aria-pressed="selected"
      @click="toggle"
    />
    <div class="squad-card__header">
      <SquadProfileSummary
        :name="squad.name"
        :profile="squad.profile"
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
          <UBadge v-if="!informational && isFull" size="sm" class="shrink-0 rounded-full px-1.5 py-0.5 font-bold" color="error" variant="solid" icon="i-ph-user-minus" :aria-label="$t('catalog.full')" />
          <UBadge v-else-if="!informational && !included" size="sm" class="shrink-0 rounded-full px-1.5 py-0.5 font-bold" :color="remainingBadge.color" variant="solid" :icon="remainingBadge.icon" :aria-label="`${$t('catalog.remaining')} ${remainingBadge.label}`" />
        </template>
      </SquadProfileSummary>
      <div v-if="!included && $slots.price" class="squad-card__price">
        <slot name="price" />
      </div>
    </div>

    <SquadProfileFacts :profile="squad.profile" presentation="member" />
    <SquadFeatures v-if="informational || squad.visible" :features="squad.features ?? []" />
    <slot />
    <div v-if="!informational" class="squad-card__footer" aria-hidden="true">
      <span>{{ included ? $t('catalog.includedSquads') : isFull ? $t('catalog.full') : selected ? $t('catalog.selectedBadge') : $t('catalog.selectSquad') }}</span>
      <UIcon :name="included || selected ? 'i-ph-check-circle' : isFull ? 'i-ph-lock-key' : 'i-ph-plus-circle'" />
    </div>
  </article>
</template>

<style scoped>
.squad-pricing-card {
  --squad-profile-tone: var(--accent);
  --squad-profile-tone-soft: color-mix(in srgb, var(--squad-profile-tone) 11%, var(--surface));
  --squad-profile-tone-line: color-mix(in srgb, var(--squad-profile-tone) 32%, var(--line));
  position: relative;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  padding: 0.6rem;
  border: 1px solid var(--line);
  border-radius: var(--radius-control);
  background: color-mix(in srgb, var(--surface-raised) 78%, transparent);
  cursor: pointer;
  transition: transform 160ms ease, background-color 160ms ease, border-color 160ms ease;
}
.squad-pricing-card:has(.squad-card__select):active { transform: scale(.995); }
.squad-pricing-card:focus-within { border-color: var(--squad-profile-tone); }
@media (prefers-reduced-motion: reduce) { .squad-pricing-card { transition: none; } .squad-pricing-card:has(.squad-card__select):active { transform: none; } }
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
.squad-card__header :deep(.squad-profile-summary--member .squad-profile-summary__name-row--with-tags) { flex-wrap: wrap; }
.squad-card__header :deep(.squad-profile-summary--member .squad-profile-summary__name-row--with-tags .squad-profile-summary__name-copy strong) { overflow: visible; overflow-wrap: anywhere; white-space: normal; }
.squad-card__price { min-width: 0; padding-top: 0.05rem; text-align: right; }
.squad-card__price > strong { display: block; white-space: nowrap; font-family: var(--font-mono); font-size: 1.1rem; }
.squad-card__footer { display: flex; align-items: center; justify-content: space-between; gap: .5rem; padding-top: .45rem; border-top: 1px solid var(--line); color: var(--squad-profile-tone, var(--text-muted)); font-size: .68rem; font-weight: 650; }
.squad-card__footer .iconify { flex: none; font-size: .95rem; }
@media (min-width: 900px) {
  .squad-pricing-card { gap: 0.4rem; padding: 0.55rem; }
  .squad-card__header { gap: 0.45rem; }
}
</style>
