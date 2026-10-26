<script setup lang="ts">
import type { DeepReadonly } from 'vue'

import type { Combo, PurchaseQuote, SquadProduct } from '@/api/types'
import type { CouponGrant } from '@/api/features'
import DualCurrencyMoney from '@/components/common/DualCurrencyMoney.vue'
import { formatBytes, formatDate } from '@/utils/format'
import { formatCatalogMoney } from '@/utils/displayCurrency'
import { useRouter } from 'vue-router'
import { useI18n } from '@/i18n'

const props = defineProps<{
  combo?: Combo
  squads: readonly SquadProduct[]
  coupon: CouponGrant | null
  quote: DeepReadonly<PurchaseQuote> | null
  quoting: boolean
  error?: string | null
  purchasing: boolean
  needsBalance: boolean
}>()
const emit = defineEmits<{ back: []; confirm: [] }>()
const router = useRouter()
const { t } = useI18n()
function couponEffect(): string {
  if (!props.coupon) return ''
  if (props.coupon.coupon.discountMode === 'percent') {
    const key = props.coupon.coupon.kind === 'purchase_once' || props.coupon.coupon.kind === 'purchase_recurring'
      ? 'coupons.effectPurchasePercent'
      : 'coupons.effectPercent'
    return t(key, { value: (Number(props.coupon.coupon.valueMinorOrBps) / 100).toFixed(2) })
  }
  return t('coupons.effectFixed', { amount: formatCatalogMoney({ currency: 'TXB', minor: props.coupon.coupon.valueMinorOrBps, display: '' }) })
}
function goToBalance(): void { void router.push({ path: '/home', query: { topUp: '1' } }) }
</script>

<template>
  <section class="catalog-checkout">
    <UButton
      class="catalog-checkout__back"
      color="neutral"
      variant="ghost"
      icon="i-ph-arrow-left"
      :label="$t('catalog.back')"
      data-haptic="navigate"
      @click="emit('back')"
    />
    <div class="section-heading section-heading--stacked">
      <h2>{{ $t('catalog.steps.review') }}</h2>
      <p>{{ $t('catalog.checkoutDescription') }}</p>
    </div>
    <div v-if="combo" class="catalog-checkout__content">
      <div class="catalog-checkout__summary">
        <div class="catalog-checkout__summary-heading">
          <span>{{ $t('catalog.purchaseSummary') }}</span>
          <UIcon name="i-ph-receipt" aria-hidden="true" />
        </div>
        <div class="catalog-checkout__line">
          <span>{{ $t('catalog.coreCombos') }}</span>
          <strong>{{ combo.name }}</strong>
        </div>
        <div v-if="squads.length" class="catalog-checkout__line">
          <span>{{ $t('catalog.optionalSquads') }}</span>
          <strong>{{ squads.map((squad) => squad.name).join($t('home.squadSeparator')) }}</strong>
        </div>
        <div class="catalog-checkout__line">
          <span>{{ $t('catalog.coupon') }}</span>
          <strong>{{ coupon?.coupon.name ?? $t('catalog.noCoupon') }}</strong>
          <small v-if="coupon">{{ couponEffect() }}</small>
        </div>
        <div class="catalog-checkout__details">
          <div v-if="quote" class="catalog-checkout__line">
            <span>{{ $t('catalog.basePrice') }}</span>
            <strong>{{ formatCatalogMoney(quote.grossPrice) }}</strong>
          </div>
          <div v-if="quote" class="catalog-checkout__line">
            <span>{{ $t('catalog.couponSavings') }}</span>
            <strong>{{ formatCatalogMoney(quote.discount) }}</strong>
          </div>
          <div v-if="quote" class="catalog-checkout__line">
            <span>{{ $t('catalog.validity') }}</span>
            <strong>{{ formatDate(quote.effectiveAt) }} {{ $t('common.rangeSeparator') }} {{ formatDate(quote.expiresAt) }}</strong>
          </div>
          <div v-if="quote" class="catalog-checkout__line">
            <span>{{ $t('catalog.accessibleNodes') }}</span>
            <strong>{{ quote.accessibleNodes.length }}</strong>
          </div>
          <div class="catalog-checkout__line">
            <span>{{ $t('catalog.traffic') }}</span>
            <strong>{{ formatBytes(combo.trafficLimitBytes) }}</strong>
          </div>
          <div class="catalog-checkout__line">
            <span>{{ $t('catalog.term') }}</span>
            <strong>{{ $t('catalog.termSummary', { days: combo.validityDays, reset: $t(`home.reset.${combo.resetStrategy}`) }) }}</strong>
          </div>
        </div>
      </div>
      <aside class="catalog-checkout__aside" :aria-label="$t('catalog.serverTotal')">
        <div class="catalog-checkout__total">
          <span>{{ quote?.queued ? $t('catalog.queuedEffectiveHint') : $t('catalog.immediateEffectiveHint') }}</span>
          <strong><DualCurrencyMoney v-if="quote" :money="quote.netPrice" /><template v-else>{{ quoting ? $t('catalog.quoting') : $t('common.notAvailable') }}</template></strong>
        </div>
      </aside>
    </div>
    <UAlert v-if="error" color="warning" variant="soft" icon="i-ph-warning-circle" :description="error" />
    <USkeleton v-if="quoting" class="h-16" />
    <div class="catalog-checkout__actions">
      <UButton v-if="needsBalance" block trailing-icon="i-ph-plus" :label="$t('catalog.addBalance')" data-haptic="navigate" @click="goToBalance" />
      <UButton v-else block class="catalog-checkout__confirm" :disabled="purchasing || !quote || quote.accessibleNodes.length === 0" :loading="purchasing" :label="purchasing ? $t('catalog.confirming') : $t('catalog.confirmPurchase')" data-haptic="confirm" @click="emit('confirm')" />
    </div>
  </section>
</template>

<style scoped>
.catalog-checkout { display: grid; gap: 0.85rem; }
.catalog-checkout__back { justify-self: start; padding-inline: 0; }
.catalog-checkout__content { display: grid; grid-template-columns: minmax(0, 1.45fr) minmax(12rem, 0.55fr); align-items: start; gap: 0.8rem; }
.catalog-checkout__summary, .catalog-checkout__aside { min-width: 0; padding: 0.9rem; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface-raised); }
.catalog-checkout__summary { display: grid; gap: 0.7rem; }
.catalog-checkout__summary-heading { display: flex; align-items: center; justify-content: space-between; gap: 0.6rem; color: var(--text-muted); font-size: 0.68rem; font-weight: 800; letter-spacing: 0.08em; text-transform: uppercase; }
.catalog-checkout__summary-heading > :last-child { color: var(--accent); font-size: 1rem; }
.catalog-checkout__details { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.7rem; padding-top: 0.15rem; }
.catalog-checkout__line { display: grid; gap: 0.22rem; padding-bottom: 0.65rem; border-bottom: 1px solid var(--line); }
.catalog-checkout__details .catalog-checkout__line { padding-bottom: 0.45rem; }
.catalog-checkout__line > span, .catalog-checkout__total > span { color: var(--text-faint); font-size: 0.68rem; }
.catalog-checkout__line strong { overflow-wrap: anywhere; font-size: 0.8rem; }
.catalog-checkout__aside { display: grid; gap: 0.8rem; position: sticky; top: 1rem; }
.catalog-checkout__total { display: grid; gap: 0.3rem; }
.catalog-checkout__total > span { line-height: 1.35; white-space: normal; }
.catalog-checkout__total strong { color: var(--accent); font-family: var(--font-mono); font-size: 1.65rem; letter-spacing: 0; }
.catalog-checkout__actions { display: grid; }
.catalog-checkout__confirm { justify-content: center; }

@media (max-width: 639px) {
  .catalog-checkout__content { grid-template-columns: 1fr; }
  .catalog-checkout__aside { position: static; }
  .catalog-checkout__details { grid-template-columns: 1fr; gap: 0.65rem; }
}
@media (min-width: 900px) {
  .catalog-checkout { gap: 0.65rem; }
  .catalog-checkout__content { gap: 0.6rem; }
  .catalog-checkout__summary, .catalog-checkout__aside { padding: 0.7rem; }
  .catalog-checkout__summary { gap: 0.55rem; }
  .catalog-checkout__details { gap: 0.5rem; }
  .catalog-checkout__line { padding-bottom: 0.45rem; }
  .catalog-checkout__details .catalog-checkout__line { padding-bottom: 0.35rem; }
}
</style>
