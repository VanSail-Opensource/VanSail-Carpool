<script setup lang="ts">
import { computed } from 'vue'
import type { AffiliateReward, AffiliateTier, CouponDefinition } from '@/api/features'
import { useI18n } from '@/i18n'

const props = defineProps<{ tier: AffiliateTier; coupons: readonly CouponDefinition[]; index: number; count: number }>()
const emit = defineEmits<{ update: [tier: AffiliateTier]; move: [offset: number]; remove: [] }>()
const { t } = useI18n()
const rewardItems = computed(() => ['none', 'coupon', 'txb', 'subscription_extension'].map((value) => ({ value, label: t(`adminAffiliates.rewardKinds.${value}`) })))
const couponItems = computed(() => props.coupons.map((coupon) => ({ value: coupon.id, label: coupon.name })))
const commissionPercent = computed({ get: () => props.tier.commissionBps / 100, set: (value: number) => update({ commissionBps: Math.round(value * 100) }) })
const txbValue = computed({ get: () => props.tier.reward.kind === 'txb' ? props.tier.reward.txbMinor / 100 : 0, set: (value: number) => reward({ kind: 'txb', txbMinor: Math.round(value * 100) }) })

function update(patch: Partial<AffiliateTier>): void { emit('update', { ...props.tier, ...patch }) }
function reward(next: AffiliateReward): void { update({ reward: next }) }
function setRewardKind(kind: AffiliateReward['kind']): void {
  if (kind === 'coupon') reward({ kind, couponId: props.coupons[0]?.id ?? '' })
  else if (kind === 'txb') reward({ kind, txbMinor: 100 })
  else if (kind === 'subscription_extension') reward({ kind, extensionDays: 1 })
  else reward({ kind })
}
</script>

<template>
  <fieldset class="affiliate-tier-editor">
    <legend>{{ $t('adminAffiliates.tierNumber', { number: index + 1 }) }}</legend>
    <div class="affiliate-tier-editor__actions">
      <UButton color="neutral" variant="ghost" square icon="i-ph-arrow-up" :disabled="index === 0" :aria-label="$t('adminAffiliates.moveUp')" @click="$emit('move', -1)" />
      <UButton color="neutral" variant="ghost" square icon="i-ph-arrow-down" :disabled="index === count - 1" :aria-label="$t('adminAffiliates.moveDown')" @click="$emit('move', 1)" />
      <UButton color="error" variant="ghost" square icon="i-ph-trash" :disabled="count === 1" :aria-label="$t('adminAffiliates.removeTier')" @click="$emit('remove')" />
    </div>
    <UFormField :label="$t('adminAffiliates.name')"><UInput :model-value="tier.name" class="w-full" :maxlength="48" @update:model-value="update({ name: String($event) })" /></UFormField>
    <UFormField :label="$t('adminAffiliates.threshold')"><UInputNumber :model-value="tier.threshold" :min="0" :step="1" @update:model-value="update({ threshold: Number($event) })" /></UFormField>
    <UFormField :label="$t('adminAffiliates.enabled')"><USwitch :model-value="tier.enabled" @update:model-value="update({ enabled: Boolean($event) })" /></UFormField>
    <UFormField :label="$t('adminAffiliates.commissionEnabled')"><USwitch :model-value="tier.commissionEnabled" @update:model-value="update({ commissionEnabled: Boolean($event), commissionBps: $event ? tier.commissionBps : 0 })" /></UFormField>
    <UFormField v-if="tier.commissionEnabled" :label="$t('adminAffiliates.commissionRate')"><UInputNumber v-model="commissionPercent" :min="0" :max="100" :step="0.01" /></UFormField>
    <UFormField :label="$t('adminAffiliates.rewardKind')"><USelect :model-value="tier.reward.kind" class="w-full" :items="rewardItems" @update:model-value="setRewardKind($event as AffiliateReward['kind'])" /></UFormField>
    <UFormField v-if="tier.reward.kind === 'coupon'" :label="$t('adminAffiliates.coupon')"><USelect :model-value="tier.reward.couponId" class="w-full" :items="couponItems" @update:model-value="reward({ kind: 'coupon', couponId: String($event) })" /></UFormField>
    <UFormField v-if="tier.reward.kind === 'txb'" :label="$t('adminAffiliates.txbReward')"><UInputNumber v-model="txbValue" :min="0.01" :step="0.01" /></UFormField>
    <UFormField v-if="tier.reward.kind === 'subscription_extension'" :label="$t('adminAffiliates.extensionDays')"><UInputNumber :model-value="tier.reward.extensionDays" :min="1" :max="3650" :step="1" @update:model-value="reward({ kind: 'subscription_extension', extensionDays: Number($event) })" /></UFormField>
  </fieldset>
</template>
