<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import { AnimatePresence, motion } from 'motion-v'

import type { SquadProduct } from '@/api/types'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useCatalog } from '@/composables/useCatalog'
import { useTelegramBackButton } from '@/composables/useTelegramBackButton'
import { motionDurations } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useSessionStore } from '@/stores/session'
import CatalogCheckout from './CatalogCheckout.vue'
import CatalogConfirmation from './CatalogConfirmation.vue'
import CatalogCouponStep from './CatalogCouponStep.vue'
import CatalogFlowControls from './CatalogFlowControls.vue'
import CatalogFlowProgress from './CatalogFlowProgress.vue'
import CatalogSquadStep from './CatalogSquadStep.vue'
import CatalogComboPricingTable from './CatalogComboPricingTable.vue'
import SquadActivationDialog from './SquadActivationDialog.vue'
import { useCatalogSquadPresentation } from './useCatalogSquadPresentation'
import { useCatalogStepPersistence } from './useCatalogStepPersistence'

defineProps<{ embedded?: boolean }>()

const activeStep = shallowRef(1)
const stepDirection = shallowRef(1)
const sessionStore = useSessionStore()
const router = useRouter()
const { reducedMotion, offset } = useMotionPreferences()

const {
  catalog,
  loading,
  purchasing,
  quoting,
  error,
  purchase,
  quote,
  quoteUsable,
  autoRenewalBlocked,
  selectedComboId,
  selectedSquadIds,
  selectedCouponGrantId,
  couponDiscarding,
  needsBalance,
  visibleCombos,
  visibleSquads,
  selectedCombo,
  selectedSquads,
  activationSquads,
  includedSquadIds,
  eligibleCoupons,
  load,
  selectCombo,
  toggleSquad,
  refreshQuote,
  discardCoupon,
  confirmPurchase,
} = useCatalog()

const { clearPersistedStep } = useCatalogStepPersistence({
  activeStep,
  userId: computed(() => sessionStore.user?.id),
  loading,
  quoting,
  quoteUsable,
  purchase,
  selectedCombo,
  selectedSquadIds,
  selectedCouponGrantId,
  refreshQuote,
})

const {
  featuredIds: featuredSquadIds,
  orderedIds: orderedSquadIds,
} = useCatalogSquadPresentation(visibleSquads, includedSquadIds, selectedComboId)

watch(activeStep, (value, previous) => {
  stepDirection.value = value >= (previous ?? value) ? 1 : -1
})

watch(autoRenewalBlocked, (blocked) => {
  if (blocked) void router.replace({ path: '/home', query: { autoRenewBlocked: '1' } })
})

const selectedCoupon = computed(
  () => eligibleCoupons.value.find((grant) => grant.id === selectedCouponGrantId.value) ?? null,
)

const nextDisabled = computed(() => {
  if (activeStep.value === 1 || activeStep.value === 2) return !selectedCombo.value
  if (activeStep.value === 3) {
    return !quoteUsable.value || !quote.value || quote.value.accessibleNodes.length === 0
  }
  return true
})

const activationOpen = shallowRef(false)
const activationTarget = shallowRef<SquadProduct | null>(null)
const activationPrompting = shallowRef(false)
let activationResolver: ((code: string | null) => void) | null = null

function goBack(): void {
  if (activeStep.value > 1) activeStep.value -= 1
}

const showCatalogBack = computed(() => !purchase.value && activeStep.value > 1)
useTelegramBackButton(showCatalogBack, goBack)

async function handlePurchase(): Promise<void> {
  if (activationPrompting.value || purchasing.value) return
  activationPrompting.value = true

  try {
    const codes: Record<string, string> = {}
    for (const squad of activationSquads.value) {
      const code = await requestActivationCode(squad)
      if (!code) return
      codes[squad.groupId] = code
    }

    if (await confirmPurchase(codes)) clearPersistedStep()
  } finally {
    activationPrompting.value = false
  }
}

function requestActivationCode(squad: SquadProduct): Promise<string | null> {
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

function goHome(): void {
  void router.push('/home')
}

async function advance(): Promise<void> {
  if (!selectedCombo.value || activeStep.value >= 4) return

  if (
    activeStep.value === 2
    && (!(await refreshQuote()) || !quote.value?.accessibleNodes.length)
  ) {
    return
  }

  if (activeStep.value === 3 && (!quoteUsable.value || quoting.value)) return
  activeStep.value += 1
}

async function handleCouponRedeemed(grantId: string | null): Promise<void> {
  await load()
  if (grantId && eligibleCoupons.value.some((grant) => grant.id === grantId)) {
    selectedCouponGrantId.value = grantId
  }
}
</script>

<template>
  <div :class="[embedded ? '' : 'page', 'page--catalog']">
    <template v-if="purchase">
      <CatalogConfirmation :purchase="purchase" @home="goHome" />
    </template>

    <template v-else>
      <header v-if="!embedded" class="page-header">
        <h1>{{ $t('catalog.title') }}</h1>
      </header>

      <CatalogFlowProgress v-model="activeStep" />

      <template v-if="loading">
        <div class="catalog-grid">
          <SkeletonBlock height="18rem" />
          <SkeletonBlock height="15rem" />
        </div>
      </template>

      <!-- Browsing the catalog must not depend on /balance being available. -->
      <template v-else-if="catalog">
        <InlineNotice v-if="error && activeStep < 4" tone="warning">
          {{ error }}
        </InlineNotice>

        <AnimatePresence mode="wait" :initial="false">
          <motion.section
            :key="activeStep"
            class="catalog-flow-step"
            :initial="{ opacity: 0, x: offset(14 * stepDirection) }"
            :animate="{ opacity: 1, x: 0 }"
            :exit="{ opacity: 0, x: reducedMotion ? 0 : -10 * stepDirection }"
            :transition="{ duration: reducedMotion ? 0.08 : motionDurations.normal, ease: 'easeOut' }"
          >
            <div v-if="activeStep === 2" class="combo-section">
              <div class="section-heading">
                <h2>{{ $t('catalog.coreCombos') }}</h2>
              </div>

              <CatalogComboPricingTable
                v-if="visibleCombos.length"
                :combos="visibleCombos"
                :selected-id="selectedComboId"
                :selected-squads="selectedSquads"
                @select="selectCombo"
              />

              <div v-else class="empty-inline">
                <div>
                  <h3>{{ $t('catalog.noCombos') }}</h3>
                  <p>{{ $t('catalog.noCombosHint') }}</p>
                </div>
                <UButton
                  color="neutral"
                  variant="outline"
                  :label="$t('common.refresh')"
                  data-haptic="refresh"
                  @click="load"
                />
              </div>
            </div>

            <CatalogSquadStep
              v-else-if="activeStep === 1"
              :squads="visibleSquads"
              :selected-ids="selectedSquadIds"
              :included-ids="includedSquadIds"
              :featured-ids="featuredSquadIds"
              :ordered-ids="orderedSquadIds"
              @toggle="toggleSquad"
            />

            <CatalogCouponStep
              v-else-if="activeStep === 3"
              v-model:coupon-grant-id="selectedCouponGrantId"
              :coupons="eligibleCoupons"
              :eligible-ids="eligibleCoupons.map((grant) => grant.id)"
              :discarding="couponDiscarding"
              :discard-coupon="discardCoupon"
              :quoting="quoting"
              @redeemed="handleCouponRedeemed"
            />

            <CatalogCheckout
              v-else-if="activeStep === 4"
              :combo="selectedCombo"
              :squads="selectedSquads"
              :coupon="selectedCoupon"
              :quote="quote"
              :quoting="quoting"
              :error="error"
              :purchasing="purchasing || activationPrompting"
              :needs-balance="needsBalance"
              @back="goBack"
              @confirm="handlePurchase"
            />
          </motion.section>
        </AnimatePresence>

        <CatalogFlowControls
          v-if="activeStep < 4"
          :show-back="activeStep > 1"
          :next-disabled="nextDisabled"
          :loading="quoting"
          :next-label="$t('catalog.continue')"
          @back="goBack"
          @next="advance"
        />
      </template>

      <div v-else class="error-state">
        <h1>{{ $t('catalog.unavailable') }}</h1>
        <p>{{ error }}</p>
        <UButton
          :label="$t('common.tryAgain')"
          data-haptic="retry"
          @click="load"
        />
      </div>
    </template>

    <SquadActivationDialog
      v-model:open="activationOpen"
      :squad="activationTarget"
      @submit="resolveActivation"
      @cancel="resolveActivation(null)"
    />
  </div>
</template>

<style scoped>
.catalog-flow-step { min-height: 14rem; }
.combo-section { display: grid; gap: 0.8rem; }
@media (min-width: 900px) {
  .page--catalog .page-header { margin-bottom: 1.2rem; }
  .catalog-flow-step { min-height: 0; }
  .combo-section { gap: 0.55rem; }
  .combo-section,
  .squad-selector { padding: 0.8rem; margin-bottom: 0.55rem; }
}
</style>
