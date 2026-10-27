<script setup lang="ts">
import type { DeepReadonly } from 'vue'
import { useRouter } from 'vue-router'

import type { Purchase, PurchaseAddonQuote, SquadProduct } from '@/api/types'
import DualCurrencyMoney from '@/components/common/DualCurrencyMoney.vue'
import { formatDate } from '@/utils/format'

defineProps<{
  squads: readonly SquadProduct[]
  quote: DeepReadonly<PurchaseAddonQuote> | null
  purchase: DeepReadonly<Purchase> | null
  quoteReady?: boolean
  quoting: boolean
  purchasing: boolean
  needsBalance: boolean
  error: string | null
}>()
const emit = defineEmits<{ back: []; confirm: []; home: [] }>()
const router = useRouter()

function goToBalance(): void {
  void router.push({ path: '/home', query: { topUp: '1' } })
}
</script>

<template>
  <section class="squad-addition-checkout" aria-live="polite">
    <template v-if="purchase">
      <div class="squad-addition-checkout__success">
        <UIcon name="i-ph-check-circle-fill" aria-hidden="true" />
        <div><strong>{{ $t('home.squadAddition.successTitle') }}</strong><p>{{ $t('home.squadAddition.successDescription') }}</p></div>
      </div>
      <UButton block trailing-icon="i-ph-house" :label="$t('catalog.returnHome')" data-haptic="navigate" @click="emit('home')" />
    </template>
    <template v-else>
      <UButton class="squad-addition-checkout__back" color="neutral" variant="ghost" icon="i-ph-arrow-left" :label="$t('catalog.back')" data-haptic="navigate" @click="emit('back')" />
      <div class="squad-addition-checkout__intro">
        <div class="section-heading section-heading--stacked"><h2>{{ $t('home.squadAddition.checkoutTitle') }}</h2><p>{{ $t('home.squadAddition.checkoutDescription') }}</p></div>
      </div>
      <div class="squad-addition-checkout__content">
        <div class="squad-addition-checkout__summary">
          <div class="squad-addition-checkout__summary-heading">
            <span>{{ $t('catalog.purchaseSummary') }}</span>
            <UIcon name="i-ph-receipt" aria-hidden="true" />
          </div>
          <div><span>{{ $t('catalog.optionalSquads') }}</span><strong>{{ squads.map((squad) => squad.name).join($t('home.squadSeparator')) }}</strong></div>
          <div v-if="quote?.grossPrice"><span>{{ $t('pricing.personalGross') }}</span><strong><DualCurrencyMoney :money="quote.grossPrice" /></strong></div>
          <div v-if="quote?.discount"><span>{{ $t('home.autoRenewalDiscount') }}</span><strong><DualCurrencyMoney :money="quote.discount" /></strong></div>
          <div v-if="quote"><span>{{ $t('home.squadAddition.validUntil') }}</span><strong>{{ formatDate(quote.expiresAt) }}</strong></div>
        </div>
        <aside class="squad-addition-checkout__aside" :aria-label="$t('home.squadAddition.proratedTotal')">
          <div class="squad-addition-checkout__total">
            <span>{{ $t('home.squadAddition.proratedTotal') }}</span>
            <strong><DualCurrencyMoney v-if="quote" :money="quote.price" /><template v-else>{{ quoting ? $t('catalog.quoting') : $t('common.notAvailable') }}</template></strong>
            <small v-if="quote">{{ $t('home.squadAddition.validUntil') }}: {{ formatDate(quote.expiresAt) }}</small>
          </div>
        </aside>
      </div>
      <UAlert v-if="error" color="warning" variant="soft" icon="i-ph-warning-circle" :description="error" />
      <USkeleton v-if="quoting" class="h-16" />
      <div class="squad-addition-checkout__actions">
        <UButton v-if="needsBalance" block trailing-icon="i-ph-plus" :label="$t('catalog.addBalance')" data-haptic="navigate" @click="goToBalance" />
        <UButton v-else block :disabled="purchasing || quoteReady === false || !quote" :loading="purchasing" trailing-icon="i-ph-check" :label="purchasing ? $t('catalog.confirming') : $t('home.squadAddition.confirm')" data-haptic="confirm" @click="emit('confirm')" />
      </div>
    </template>
  </section>
</template>

<style scoped>
.squad-addition-checkout { display: grid; gap: 0.85rem; }
.squad-addition-checkout__back { justify-self: start; padding-inline: 0; }
.squad-addition-checkout__intro .section-heading { margin-bottom: 0; }
.squad-addition-checkout__total span, .squad-addition-checkout__total small { color: var(--text-faint); font-size: 0.68rem; }
.squad-addition-checkout__content { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(10rem, 0.65fr); align-items: start; gap: 0.75rem; }
.squad-addition-checkout__summary, .squad-addition-checkout__aside { min-width: 0; padding: 0.85rem; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface-raised); }
.squad-addition-checkout__summary { display: grid; gap: 0.7rem; }
.squad-addition-checkout__summary-heading { display: flex; align-items: center; justify-content: space-between; gap: 0.6rem; color: var(--text-muted); font-size: 0.68rem; font-weight: 800; letter-spacing: 0.08em; text-transform: uppercase; }
.squad-addition-checkout__summary-heading > :last-child { color: var(--accent); font-size: 1rem; }
.squad-addition-checkout__summary > div:not(.squad-addition-checkout__summary-heading) { display: grid; gap: 0.25rem; padding-bottom: 0.65rem; border-bottom: 1px solid var(--line); }
.squad-addition-checkout__summary > div:last-child { padding-bottom: 0; border-bottom: 0; }
.squad-addition-checkout__summary span { color: var(--text-faint); font-size: 0.68rem; }
.squad-addition-checkout__summary strong { overflow-wrap: anywhere; font-size: 0.8rem; }
.squad-addition-checkout__aside { position: sticky; top: 0.75rem; }
.squad-addition-checkout__total { display: grid; gap: 0.3rem; }
.squad-addition-checkout__total strong { color: var(--accent); font-family: var(--font-mono); font-size: 1.55rem; }
.squad-addition-checkout__total small { line-height: 1.35; }
.squad-addition-checkout__actions { display: grid; }
.squad-addition-checkout__success { display: flex; align-items: flex-start; gap: 0.75rem; padding: 0.9rem; border: 1px solid var(--success); border-radius: var(--radius-panel); background: var(--success-soft); }
.squad-addition-checkout__success > :first-child { flex: 0 0 auto; color: var(--success); font-size: 1.5rem; }
.squad-addition-checkout__success p { margin: 0.3rem 0 0; color: var(--text-muted); font-size: 0.8rem; }

@media (max-width: 639px) {
  .squad-addition-checkout__content { grid-template-columns: 1fr; }
  .squad-addition-checkout__aside { position: static; }
}
</style>
