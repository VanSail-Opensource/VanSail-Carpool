<script setup lang="ts">
import { computed } from 'vue'
import type { RewardKind } from '@/api/features'
import { useI18n } from '@/i18n'
import type { PrizeDraft } from './drawDraft'

const props = defineProps<{
  prize: PrizeDraft
  drawKind: 'instant' | 'raffle'
  comboItems: { value: string; label: string }[]
  squadItems: { value: string; label: string }[]
  removable: boolean
}>()
const emit = defineEmits<{ change: [patch: Partial<PrizeDraft>]; remove: [] }>()
const { t } = useI18n()
const random = computed(() => ['txb_delta', 'subscription_extension', 'traffic_grant', 'balance_multiplier',
  'coupon_recurring', 'coupon_once'].includes(props.prize.kind))
const signed = computed(() => props.prize.kind === 'txb_delta' || props.prize.kind === 'traffic_grant'
  ? Number(props.prize.rangeMin) < 0 && Number(props.prize.rangeMax) > 0
  : props.prize.kind === 'balance_multiplier' && Number(props.prize.rangeMin) < 1 && Number(props.prize.rangeMax) > 1)
const coupon = computed(() => props.prize.kind === 'coupon_recurring' || props.prize.kind === 'coupon_once')
const rangeUnit = computed(() => {
  if (props.prize.kind === 'traffic_grant') return 'GiB'
  if (props.prize.kind === 'subscription_extension') return t('adminDrawRedesign.unitHours')
  if (props.prize.kind === 'balance_multiplier') return '×'
  if (coupon.value && props.prize.discountMode === 'percent') return '%'
  return 'TXB'
})
const rewardItems = computed(() => [
  ['none', 'none'], ['txb_delta', 'txb'], ['subscription_extension', 'extension'],
  ['traffic_grant', 'traffic'], ['traffic_reset', 'reset'], ['balance_multiplier', 'multiplier'],
  ['coupon_recurring', 'couponRecurring'], ['coupon_once', 'couponOnce'],
  ['entitlement_grant', 'entitlement'], ['squad_access', 'squads'],
  ['core_combo_switch', 'comboSwitch'],
].map(([value, label]) => ({ value, label: t('adminDrawRedesign.' + label) })))
const distributionItems = computed(() => [
  { value: 'uniform', label: t('adminDrawRedesign.uniform') },
  { value: 'gaussian', label: t('adminDrawRedesign.gaussian') },
  { value: 'power_law', label: t('adminDrawRedesign.powerLaw') },
])
const discountItems = computed(() => [
  { value: 'fixed', label: t('adminDrawRedesign.fixed') },
  { value: 'percent', label: t('adminDrawRedesign.percent') },
])
function patch(field: keyof PrizeDraft, value: unknown): void {
  emit('change', { [field]: value } as Partial<PrizeDraft>)
}
</script>

<template>
  <article class="draw-prize-row">
    <div class="draw-prize-row__top">
      <UFormField :label="t('adminDrawRedesign.prizeName')" class="draw-prize-row__name" required>
        <UInput :model-value="prize.name" class="w-full" :maxlength="80" @update:model-value="patch('name', String($event))" />
      </UFormField>
      <UButton
        color="error" variant="ghost" icon="i-ph-trash" square :disabled="!removable"
        :aria-label="t('adminDrawRedesign.remove')" @click="emit('remove')"
      />
    </div>
    <div class="draw-prize-row__grid">
      <UFormField :label="drawKind === 'instant' ? t('adminDrawRedesign.probability') : t('adminDrawRedesign.stock')" required>
        <UInput
          v-if="drawKind === 'instant'" :model-value="prize.probability" class="w-full" inputmode="decimal"
          @update:model-value="patch('probability', String($event))"
        />
        <UInput
          v-else :model-value="prize.stock" class="w-full" inputmode="numeric"
          @update:model-value="patch('stock', String($event))"
        />
      </UFormField>
      <UFormField :label="t('adminDrawRedesign.reward')" required>
        <USelect
          :model-value="prize.kind" class="w-full" :items="rewardItems"
          @update:model-value="patch('kind', String($event) as RewardKind)"
        />
      </UFormField>
      <UFormField v-if="coupon" :label="t('adminDrawRedesign.discountMode')">
        <USelect
          :model-value="prize.discountMode" class="w-full" :items="discountItems"
          @update:model-value="patch('discountMode', String($event))"
        />
      </UFormField>
      <template v-if="random">
        <UFormField :label="t('adminDrawRedesign.rangeMin') + ' (' + rangeUnit + ')'" required>
          <UInput
            :model-value="prize.rangeMin" class="w-full" inputmode="decimal"
            @update:model-value="patch('rangeMin', String($event))"
          />
        </UFormField>
        <UFormField :label="t('adminDrawRedesign.rangeMax') + ' (' + rangeUnit + ')'" required>
          <UInput
            :model-value="prize.rangeMax" class="w-full" inputmode="decimal"
            @update:model-value="patch('rangeMax', String($event))"
          />
        </UFormField>
        <UFormField :label="t('adminDrawRedesign.distribution')">
          <USelect
            :model-value="prize.distribution" class="w-full" :items="distributionItems"
            @update:model-value="patch('distribution', String($event))"
          />
        </UFormField>
        <UFormField v-if="signed" :label="t('adminDrawRedesign.positiveChance')">
          <UInput
            :model-value="prize.positiveChance" class="w-full" inputmode="decimal"
            @update:model-value="patch('positiveChance', String($event))"
          />
        </UFormField>
      </template>
      <UFormField
        v-if="prize.kind === 'entitlement_grant' || prize.kind === 'core_combo_switch'"
        :label="t('adminDrawRedesign.combo')" required
      >
        <USelectMenu
          :model-value="prize.comboId" class="w-full" :items="comboItems" value-key="value" label-key="label"
          @update:model-value="patch('comboId', String($event))"
        />
      </UFormField>
      <UFormField
        v-if="prize.kind === 'entitlement_grant' || prize.kind === 'squad_access'"
        :label="t('adminDrawRedesign.squadSelection')"
      >
        <USelectMenu
          :model-value="prize.squadUuids" class="w-full" :items="squadItems" value-key="value" label-key="label"
          multiple @update:model-value="patch('squadUuids', $event)"
        />
      </UFormField>
      <template v-if="prize.kind === 'entitlement_grant'">
        <UFormField :label="t('adminDrawRedesign.renewalPrice')" required>
          <UInput
            :model-value="prize.renewalPrice" class="w-full" inputmode="decimal"
            @update:model-value="patch('renewalPrice', String($event))"
          />
        </UFormField>
        <UFormField :label="t('adminDrawRedesign.trafficLimit')" required>
          <UInput
            :model-value="prize.trafficLimitGiB" class="w-full" inputmode="numeric"
            @update:model-value="patch('trafficLimitGiB', String($event))"
          />
        </UFormField>
      </template>
      <UFormField v-if="prize.kind === 'traffic_grant'" :label="t('adminDrawRedesign.includeRenewal')">
        <USwitch :model-value="prize.includeInRenewal" @update:model-value="patch('includeInRenewal', Boolean($event))" />
      </UFormField>
    </div>
  </article>
</template>

<style scoped>
.draw-prize-row { display: grid; gap: 0.65rem; padding: 0.85rem 0; border-top: 1px solid var(--line); }
.draw-prize-row__top { display: flex; align-items: end; gap: 0.5rem; }
.draw-prize-row__name { flex: 1; min-width: 0; }
.draw-prize-row__grid { display: grid; gap: 0.7rem; grid-template-columns: minmax(0, 1fr); }
@media (min-width: 640px) { .draw-prize-row__grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
