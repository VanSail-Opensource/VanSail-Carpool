<script setup lang="ts">
import { useRouter } from 'vue-router'
import { subscriptionApi } from '@/api/subscription'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useI18n } from '@/i18n'
import UptimeBar from './UptimeBar.vue'
import { useSubscriptionData } from './useSubscriptionData'

const { value: summary, loading, error, refresh } = useSubscriptionData(subscriptionApi.summary)
const { t } = useI18n()
const router = useRouter()

function openSubscription(): void {
  // Keep the native Telegram WebView from treating an anchor as a new launch URL.
  void router.push({ name: 'subscription' }).catch(() => undefined)
}
</script>

<template>
  <section class="section-block home-uptime">
    <SkeletonBlock v-if="loading" height="6rem" />
    <UptimeBar v-else-if="summary?.activeCombo && summary.monitoringEnabled && !error" :timeline="summary" :label="t('uptime.summary')" />
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <p v-else-if="summary?.activeCombo && summary.monitoringEnabled && summary.errorCode" class="home-uptime__hint">{{ t('uptime.notAvailable') }}</p>
    <div class="home-uptime__entrance" :class="{ 'home-uptime__entrance--only': summary && (!summary.activeCombo || !summary.monitoringEnabled) }">
      <UButton type="button" color="neutral" variant="link" icon="i-ph-link-bold" :label="t('subscription.title')" data-haptic="navigate" @click="openSubscription" />
      <UButton v-if="error" color="neutral" variant="ghost" :label="t('common.tryAgain')" @click="refresh()" />
    </div>
  </section>
</template>

<style scoped>
.home-uptime { display: grid; gap: 0.5rem; }
.home-uptime__entrance { display: grid; justify-items: center; gap: 0.5rem; border-top: 1px solid var(--line); padding-top: 0.45rem; }
.home-uptime__entrance--only { border-top: 0; padding-top: 0; }
.home-uptime__entrance :deep(button) { min-height: 44px; }
.home-uptime__hint { margin: 0; color: var(--text-muted); font-size: 0.75rem; }
</style>
