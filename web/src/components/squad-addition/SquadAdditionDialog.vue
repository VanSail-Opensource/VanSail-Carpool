<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import PricingQuoteNotice from '@/components/common/PricingQuoteNotice.vue'
import { useRouter } from 'vue-router'

import type { Purchase, SquadProduct } from '@/api/types'
import CatalogCouponStep from '@/components/catalog/CatalogCouponStep.vue'
import CatalogSquadStep from '@/components/catalog/CatalogSquadStep.vue'
import SquadActivationDialog from '@/components/catalog/SquadActivationDialog.vue'
import { useCatalogSquadPresentation } from '@/components/catalog/useCatalogSquadPresentation'
import { useSquadAddition } from '@/composables/useSquadAddition'
import { useMotionPreferences } from '@/composables/useMotionPreferences'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ active: Purchase }>()
const emit = defineEmits<{ changed: [] }>()
const router = useRouter()
const { reducedMotion } = useMotionPreferences()
const step = shallowRef(1)
const activationOpen = shallowRef(false)
const activationTarget = shallowRef<SquadProduct | null>(null)
const activationPrompting = shallowRef(false)
let activationResolver: ((code: string | null) => void) | null = null
const emptyIncludedIds = computed<readonly string[]>(() => [])
const comboId = computed<string | null>(() => props.active.comboId)
const activeSquadUuids = computed(() => props.active.squadUuids)
const {
  couponGrants, selectedCouponGrantId, eligibleCoupons,
  selectedSquadIds, quote, purchase, loading, quoting, purchasing, needsBalance, error,
  visibleSquads, selectedSquads, activationSquads, quoteUsable, quoteStatus, reset, toggleSquad, load, refreshQuote, confirmPurchase,
} = useSquadAddition({ comboId: () => props.active.comboId, purchaseId: () => props.active.id, activeSquadUuids: () => activeSquadUuids.value })
const { featuredIds, orderedIds } = useCatalogSquadPresentation(visibleSquads, emptyIncludedIds, comboId)

watch([open, () => props.active.id], ([visible]) => {
  if (!visible) {
    reset()
    if (activationResolver) resolveActivation(null)
    return
  }
  step.value = 1
  reset()
  void load()
}, { immediate: true })

async function continueToCheckout(): Promise<void> {
  if (!selectedSquadIds.value.length || !(await refreshQuote())) return
  step.value = 2
}

async function requestActivationCode(squad: SquadProduct): Promise<string | null> {
  activationTarget.value = squad
  activationOpen.value = true
  return new Promise((resolve) => { activationResolver = resolve })
}

function resolveActivation(code: string | null): void {
  const resolve = activationResolver
  activationResolver = null
  activationOpen.value = false
  activationTarget.value = null
  resolve?.(code)
}

async function confirm(): Promise<void> {
  if (activationPrompting.value) return
  activationPrompting.value = true
  try {
    const codes: Record<string, string> = {}
    for (const squad of activationSquads.value) {
      const code = await requestActivationCode(squad)
      if (!code) return
      codes[squad.groupId] = code
    }
    if (await confirmPurchase(codes)) emit('changed')
  } finally {
    activationPrompting.value = false
  }
}

function close(): void {
  open.value = false
}

function goHome(): void {
  close()
  void router.push('/home')
}
</script>

<template>
  <UModal v-model:open="open" :title="$t('home.squadAddition.title')" :description="$t('home.squadAddition.description')" :close="false" :ui="{ content: 'tg-glass-modal tg-glass-surface squad-addition-dialog__modal' + (reducedMotion ? ' squad-addition-dialog__modal--reduced' : ''), header: 'tg-overlay-header--centered', wrapper: 'tg-overlay-copy--centered', footer: 'justify-end flex-wrap' }" scrollable>
    <template #body>
      <div class="squad-addition-dialog">
        <UStepper :model-value="step - 1" :items="[{ title: $t('home.squadAddition.steps.choose') }, { title: $t('home.squadAddition.steps.checkout') }]" :linear="true" disabled size="sm" aria-hidden="true" />
        <p role="status" class="sr-only">{{ $t('home.squadAddition.progress', { current: step, total: 2, title: $t(step === 1 ? 'home.squadAddition.steps.choose' : 'home.squadAddition.steps.checkout') }) }}</p>
        <Transition name="squad-addition-step" :mode="reducedMotion ? undefined : 'out-in'" :css="!reducedMotion">
          <div :key="loading ? 'loading' : step">
            <USkeleton v-if="loading" class="h-48" />
            <template v-else-if="step === 1">
              <CatalogSquadStep :squads="visibleSquads" :selected-ids="selectedSquadIds" :included-ids="emptyIncludedIds" :featured-ids="featuredIds" :ordered-ids="orderedIds" @toggle="toggleSquad" />
              <CatalogCouponStep v-model:coupon-grant-id="selectedCouponGrantId" :coupons="couponGrants" :eligible-ids="eligibleCoupons.map(grant => grant.id)" :quoting="quoting" @redeemed="load()" />
            </template>
            <SquadAdditionCheckout v-else :squads="selectedSquads" :quote="quote" :quote-ready="quoteUsable" :purchase="purchase" :quoting="quoting" :purchasing="purchasing || activationPrompting" :needs-balance="needsBalance" :error="error" @back="step = 1" @confirm="confirm" @home="goHome" />
          </div>
        </Transition>
        <PricingQuoteNotice :state="quoteStatus" :valid-until="quote?.quoteValidUntil" @retry="refreshQuote()" />
        <UAlert v-if="step === 1 && error" color="warning" variant="soft" icon="i-ph-warning-circle" :description="error" />
      </div>
    </template>
    <template v-if="step === 1" #footer>
      <UButton color="neutral" variant="outline" :label="$t('common.cancel')" data-haptic="dismiss" @click="close" />
      <UButton :disabled="loading || !selectedSquadIds.length" :label="$t('catalog.continue')" data-haptic="confirm" @click="continueToCheckout" />
    </template>
  </UModal>
  <SquadActivationDialog v-model:open="activationOpen" :squad="activationTarget" @submit="resolveActivation" @cancel="resolveActivation(null)" />
</template>

<style scoped>
.squad-addition-dialog { display: grid; gap: 1rem; min-width: 0; padding-bottom: max(0.25rem, env(safe-area-inset-bottom)); }
:global(.squad-addition-dialog__modal) { width: min(52rem, 100%); }
.squad-addition-step-enter-active, .squad-addition-step-leave-active { transition: opacity 160ms ease-out, transform 160ms ease-out; }
.squad-addition-step-enter-from, .squad-addition-step-leave-to { opacity: 0; transform: translateY(4px); }
:global(.squad-addition-dialog__modal[data-state='open']) { animation: squad-addition-open 160ms ease-out; }
:global(.squad-addition-dialog__modal[data-state='closed']) { animation: squad-addition-close 160ms ease-out; }
@keyframes squad-addition-open { from { opacity: 0; } to { opacity: 1; } }
@keyframes squad-addition-close { from { opacity: 1; } to { opacity: 0; } }
@media (prefers-reduced-motion: reduce) { :global(.squad-addition-dialog__modal) { animation: none; } }
:global(.squad-addition-dialog__modal--reduced[data-state]) { animation: none; }

@media (max-width: 639px) {
  :global(.squad-addition-dialog__modal) { width: 100%; }
}
</style>
