<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useCommunityAccess } from '@/composables/useCommunityAccess'
import { useDashboard } from '@/composables/useDashboard'
import { useI18n } from '@/i18n'
import BalanceHero from './BalanceHero.vue'
import ComingSoonLinks from './ComingSoonLinks.vue'
import EntitlementSummary from './EntitlementSummary.vue'
import HomeUptimePanel from '@/components/subscription/HomeUptimePanel.vue'
import UsagePanel from './UsagePanel.vue'
import MemberLiveMetrics from './MemberLiveMetrics.vue'

const { dashboard, loading, error, usageRatio, catalogNodes, activeSquadNames, load } = useDashboard()
const { activeCombo: hasValidCombo, refresh: refreshCommunityAccess } = useCommunityAccess()
const queuedCancellationNotice = shallowRef(false)
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const reissueOrderId = computed(() => typeof route.query.reissue === 'string' && route.query.reissue ? route.query.reissue : undefined)
const topUpRequested = computed(() => route.query.topUp === '1' && !reissueOrderId.value)
watch(() => route.query.revoke, value => { if (value === '1') void router.replace({ path: '/subscription', query: { revoke: '1' } }) }, { immediate: true })
watch(() => route.query.addSquads, value => { if (value === '1') void router.replace({ path: '/settings', query: { addSquads: '1' } }) }, { immediate: true })
const catalogBlocked = computed(() => route.query.autoRenewBlocked === '1')
const autoRenewalFailureMessage = computed(() => {
  const reason = dashboard.value?.autoRenewalFailure?.reason
  if (!reason) return null
  const key = `home.autoRenewalReason.${reason}`
  const localized = t(key)
  return localized === key ? t('home.autoRenewalFailureGeneric') : localized
})

async function handlePaid(): Promise<void> {
  await load({ quiet: true })
  await refreshCommunityAccess()
}

async function handleQueuedCancelled(): Promise<void> {
  queuedCancellationNotice.value = true
  await load({ quiet: true })
}

async function handleAutoRenewalChanged(): Promise<void> {
  await load({ quiet: true })
}


function consumeTopUpRequest(): void {
  if (!topUpRequested.value) return
  const query = { ...route.query }
  delete query.topUp
  void router.replace({ name: 'home', query })
}

function consumeReissueRequest(): void {
  if (!reissueOrderId.value) return
  const query = { ...route.query }
  delete query.reissue
  void router.replace({ name: 'home', query })
}

</script>

<template>
  <div class="page page--home">
    <template v-if="loading">
      <SkeletonBlock height="13rem" />
      <div class="content-grid">
        <SkeletonBlock height="18rem" />
        <SkeletonBlock height="18rem" />
      </div>
    </template>
    <template v-else-if="dashboard">
      <h1 class="sr-only home-page__title">{{ $t('nav.home') }}</h1>
      <div class="home-layout">
        <BalanceHero
          :balance="dashboard.balance"
          :open-top-up="topUpRequested"
          :reissue-order-id="reissueOrderId"
          @paid="handlePaid"
          @top-up-request-consumed="consumeTopUpRequest"
          @reissue-request-consumed="consumeReissueRequest"
        />
        <div class="home-layout__columns">
          <div class="home-alerts">
            <InlineNotice v-if="catalogBlocked" tone="warning">{{ $t('home.autoRenewalCatalogBlocked') }}</InlineNotice>
            <InlineNotice v-if="autoRenewalFailureMessage" tone="warning" :title="$t('home.autoRenewalFailureTitle')">{{ autoRenewalFailureMessage }}</InlineNotice>
            <InlineNotice v-if="queuedCancellationNotice" tone="success">{{ $t('home.queuedCancelled') }}</InlineNotice>
          </div>
          <HomeUptimePanel v-if="hasValidCombo" />
          <MemberLiveMetrics v-if="hasValidCombo" />
          <UsagePanel
            v-if="dashboard.statistics"
            class="home-layout__usage"
            :statistics="dashboard.statistics"
            :ratio="usageRatio"
            :stale="dashboard.statisticsStale"
            :fetched-at="dashboard.fetchedAt"
            :catalog-nodes="catalogNodes"
          />
          <section v-else class="section-block home-usage home-usage--empty home-layout__usage empty-inline">
            <div><h3>{{ $t('dashboard.noStatistics') }}</h3><p>{{ $t('dashboard.statisticsPending') }}</p></div>
          </section>
          <EntitlementSummary
            :active="dashboard.activePurchase"
            :queued="dashboard.queuedPurchase"
            :squad-names="activeSquadNames"
            @queued-cancelled="handleQueuedCancelled"
            @auto-renewal-changed="handleAutoRenewalChanged"
          />
          <ComingSoonLinks :has-valid-combo="hasValidCombo" />
        </div>
      </div>
    </template>
    <div v-else class="error-state">
      <h1>{{ $t('dashboard.unavailable') }}</h1>
      <p>{{ error ?? $t('dashboard.loadFailed') }}</p>
      <UButton :label="$t('common.tryAgain')" data-haptic="retry" @click="load()" />
    </div>
  </div>
</template>
