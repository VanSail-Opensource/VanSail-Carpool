<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

import { useAffiliateCentre } from '@/composables/useAffiliateCentre'
import { useTelegramBackButton } from '@/composables/useTelegramBackButton'
import InlineNotice from '@/components/common/InlineNotice.vue'
import AffiliateReferralList from './AffiliateReferralList.vue'
import AffiliateSummary from './AffiliateSummary.vue'
import AffiliateTierProgress from './AffiliateTierProgress.vue'

const router = useRouter()
const state = useAffiliateCentre()
useTelegramBackButton(computed(() => true), async () => { await router.push('/home') })
</script>

<template>
  <div class="page affiliate-page">
    <header class="affiliate-heading">
      <p class="eyebrow">{{ $t('dashboard.aroundTx') }}</p>
      <h1>{{ $t('affiliates.title') }}</h1>
      <p>{{ $t('affiliates.subtitle') }}</p>
    </header>
    <template v-if="state.loading.value">
      <USkeleton class="h-44 w-full" /><USkeleton class="h-40 w-full" /><USkeleton class="h-56 w-full" />
    </template>
    <template v-else-if="state.overview.value && state.referrals.value">
      <InlineNotice v-if="state.error.value" tone="warning">{{ state.error.value }}</InlineNotice>
      <AffiliateSummary :overview="state.overview.value" />
      <AffiliateTierProgress :progress="state.overview.value.tierProgress" />
      <AffiliateReferralList
        :page="state.referrals.value"
        :loading="state.referralsLoading.value"
        @update:page="state.setPage"
      />
    </template>
    <div v-else class="error-state">
      <h2>{{ $t('affiliates.unavailable') }}</h2><p>{{ state.error.value }}</p>
      <UButton icon="i-ph-arrows-clockwise" :label="$t('common.tryAgain')" data-haptic="retry" @click="state.load" />
    </div>
  </div>
</template>
