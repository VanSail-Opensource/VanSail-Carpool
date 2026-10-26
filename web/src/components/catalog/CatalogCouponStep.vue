<script setup lang="ts">
import { computed, shallowRef } from 'vue'

import type { CouponGrant } from '@/api/features'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useCouponRedemption } from '@/composables/useCouponRedemption'
import { useI18n } from '@/i18n'
import { formatCatalogMoney } from '@/utils/displayCurrency'
import { selectionHaptic } from '@/utils/telegram'

const couponGrantId = defineModel<string | null>('couponGrantId', { required: true })
const props = defineProps<{
  coupons: readonly CouponGrant[]
  eligibleIds: readonly string[]
  discarding?: boolean
  quoting?: boolean
  discardCoupon?: (grantID: string) => Promise<boolean>
}>()
const emit = defineEmits<{ redeemed: [grantId: string | null] }>()

const code = shallowRef('')
const pendingDiscard = shallowRef<CouponGrant | null>(null)
const hasEligibleCoupons = computed(() => props.coupons.some((grant) => props.eligibleIds.includes(grant.id)))
const { redeeming, error, message, redeem } = useCouponRedemption()
const { t } = useI18n()

function couponEffect(grant: CouponGrant): string {
  if (grant.coupon.discountMode === 'percent') {
    const key = grant.coupon.kind === 'purchase_once' || grant.coupon.kind === 'purchase_recurring'
      ? 'coupons.effectPurchasePercent'
      : 'coupons.effectPercent'
    return t(key, { value: (Number(grant.coupon.valueMinorOrBps) / 100).toFixed(2) })
  }
  return t('coupons.effectFixed', { amount: formatCatalogMoney({ currency: 'TXB', minor: grant.coupon.valueMinorOrBps, display: '' }) })
}

async function submit(): Promise<void> {
  const result = await redeem(code.value)
  if (!result) return
  code.value = ''
  emit('redeemed', result.grant?.id ?? null)
}

async function confirmDiscard(): Promise<void> {
  const grant = pendingDiscard.value
  if (!grant || !props.discardCoupon) return
  if (await props.discardCoupon(grant.id)) pendingDiscard.value = null
}

function selectCoupon(grantId: string | null): void {
  if (couponGrantId.value === grantId) return
  couponGrantId.value = grantId
  selectionHaptic()
}
</script>

<template>
  <section class="catalog-coupon-step">
    <div class="section-heading section-heading--stacked">
      <h2>{{ $t('catalog.coupon') }}</h2>
      <p>{{ $t('catalog.couponStepHint') }}</p>
    </div>
    <form class="catalog-coupon-form" @submit.prevent="submit">
      <UFormField :label="$t('catalog.addCoupon')">
        <UInput v-model.trim="code" icon="i-ph-ticket" :placeholder="$t('coupons.codePlaceholder')" autocomplete="off" />
      </UFormField>
      <UButton type="submit" color="neutral" variant="outline" :loading="redeeming" :disabled="redeeming || !code" :label="redeeming ? $t('coupons.redeeming') : $t('coupons.redeem')" data-haptic="action" />
    </form>
    <InlineNotice v-if="message" tone="success">{{ message }}</InlineNotice>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <div class="catalog-coupon-list">
      <UButton class="catalog-coupon-choice" :class="{ 'catalog-coupon-choice--selected': couponGrantId === null }" color="neutral" variant="ghost" :aria-pressed="couponGrantId === null" @click="selectCoupon(null)">
        <span><strong>{{ $t('catalog.noCoupon') }}</strong><small>{{ $t('catalog.noCouponHint') }}</small></span>
        <UIcon v-if="couponGrantId === null" name="i-ph-check-bold" />
      </UButton>
      <div v-for="grant in props.coupons" :key="grant.id" class="catalog-coupon-choice-row">
        <UButton class="catalog-coupon-choice" :class="{ 'catalog-coupon-choice--selected': couponGrantId === grant.id, 'catalog-coupon-choice--ineligible': !eligibleIds.includes(grant.id) }" color="neutral" variant="ghost" :aria-pressed="couponGrantId === grant.id" :disabled="!eligibleIds.includes(grant.id)" @click="selectCoupon(grant.id)">
          <span><strong>{{ grant.coupon.name }}</strong><small>{{ eligibleIds.includes(grant.id) ? couponEffect(grant) : $t('catalog.couponIneligible') }}</small></span>
          <UIcon v-if="couponGrantId === grant.id" name="i-ph-check-bold" />
        </UButton>
        <UButton v-if="discardCoupon" color="error" variant="ghost" square icon="i-ph-trash" :aria-label="$t('coupons.discardTitle', { name: grant.coupon.name })" data-haptic="open" @click="pendingDiscard = grant" />
      </div>
    </div>
    <p v-if="!hasEligibleCoupons" class="field-hint">{{ $t('catalog.noEligibleCoupons') }}</p>
    <p v-if="quoting" class="field-hint" aria-live="polite">{{ $t('catalog.couponQuoteRefreshing') }}</p>
    <ConfirmDialog :open="Boolean(pendingDiscard)" :title="$t('coupons.discardTitle', { name: pendingDiscard?.coupon.name ?? '' })" :description="$t('coupons.discardDescription')" :confirm-label="$t('coupons.discardConfirm')" :busy="Boolean(discarding)" danger @update:open="!$event && (pendingDiscard = null)" @confirm="confirmDiscard" />
  </section>
</template>

<style scoped>
.catalog-coupon-step, .catalog-coupon-list { display: grid; gap: 0.75rem; }
.catalog-coupon-form { display: grid; grid-template-columns: minmax(0, 22rem) auto; align-items: end; justify-content: start; gap: 0.55rem; }
.catalog-coupon-choice-row { display: flex; align-items: center; gap: 0.25rem; }
.catalog-coupon-choice { min-height: 60px; flex: 1; display: flex; align-items: center; justify-content: space-between; gap: 0.7rem; padding: 0.65rem; border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface-raised); text-align: left; }
.catalog-coupon-choice--selected { border-color: var(--accent); background: var(--accent-soft); }
.catalog-coupon-choice--ineligible { opacity: 0.58; }
.catalog-coupon-choice > span { min-width: 0; }
.catalog-coupon-choice strong, .catalog-coupon-choice small { display: block; overflow-wrap: anywhere; }
.catalog-coupon-choice strong { font-size: 0.78rem; }
.catalog-coupon-choice small { margin-top: 0.18rem; color: var(--text-faint); font-family: var(--font-mono); font-size: 0.64rem; }

@media (min-width: 900px) {
  .catalog-coupon-list { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .catalog-coupon-step, .catalog-coupon-list { gap: 0.55rem; }
  .catalog-coupon-choice { padding: 0.5rem; }
}

@media (max-width: 600px) {
  .catalog-coupon-form { grid-template-columns: 1fr; align-items: stretch; }
}
</style>
