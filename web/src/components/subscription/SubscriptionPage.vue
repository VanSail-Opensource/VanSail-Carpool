<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api/client'
import { subscriptionApi } from '@/api/subscription'
import InlineNotice from '@/components/common/InlineNotice.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import SubscriptionPanel from '@/components/dashboard/SubscriptionPanel.vue'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { useI18n } from '@/i18n'
import { notifyHaptic } from '@/utils/telegram'
import HostLinkRow from './HostLinkRow.vue'
import { useSubscriptionData } from './useSubscriptionData'
import SubscriptionFormats from './SubscriptionFormats.vue'

const { value: subscription, loading, error, refresh } = useSubscriptionData(subscriptionApi.subscription, 60_000, 'subscription.loadFailed', value => value?.ready === false ? 2000 : 60_000)
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const revokeRequested = computed(() => route.query.revoke === '1')
const command = useDurableCommand({
  errorKey: 'errors.subscriptionRevoke',
  onTerminal: async receipt => {
    if (receipt.status === 'succeeded') { await refresh(true); notifyHaptic('success') }
    else notifyHaptic('error')
  },
})
const { busy, blocksMutations, receipt, checking, error: revokeError, refresh: refreshRevoke } = command
async function revoke(): Promise<void> {
  if (revokeRequested.value) consumeRevoke()
  if (!await command.execute('subscription-revoke', 'subscription-revoke', api.revokeSubscription)) notifyHaptic('error')
}
function consumeRevoke(): void {
  const query = { ...route.query }
  delete query.revoke
  void router.replace({ name: 'subscription', query })
}
</script>

<template>
  <div class="page page--subscription">
    <header class="page-header"><h1>{{ t('subscription.title') }}</h1><p>{{ t('subscription.copy') }}</p></header>
    <OperationStatusNotice v-if="receipt?.status !== 'succeeded'" :receipt="receipt" :error="revokeError" :checking="checking" @refresh="refreshRevoke" />
    <InlineNotice v-if="receipt?.status === 'succeeded'" tone="success" :title="t('dashboard.linkReplaced')">{{ t('dashboard.previousLinkInvalid') }}</InlineNotice>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <UButton v-if="error" color="neutral" variant="outline" :label="t('common.tryAgain')" @click="refresh()" />
    <div v-if="loading" class="subscription-page__loading"><SkeletonBlock height="11rem" /><SkeletonBlock height="10rem" /></div>
    <template v-else-if="subscription?.subscriptionUrl">
      <SubscriptionPanel :subscription-url="subscription.subscriptionUrl" :revoking="busy" :revoke-blocked="blocksMutations" :open-revoke="revokeRequested" @revoke="revoke" @revoke-request-consumed="consumeRevoke" />
      <SubscriptionFormats :key="subscription.subscriptionUrl ?? ''" />
      <section class="section-block">
        <div class="section-heading"><h2>{{ t('subscription.hosts') }}</h2></div>
        <ul v-if="subscription.hosts.length" class="subscription-page__hosts"><HostLinkRow v-for="host in subscription.hosts" :key="host.uuid" :host="host" @saved="refresh()" /></ul>
        <p v-else class="subscription-page__empty">{{ t('subscription.noHosts') }}</p>
      </section>
    </template>
    <section v-else-if="subscription?.ready === false" class="section-block grid gap-3"><h2>{{ t('subscription.firstSyncPending') }}</h2><p class="text-sm text-muted">{{ t('subscription.firstSyncHint') }}</p><UButton variant="outline" color="neutral" :label="t('common.refresh')" @click="refresh()" /></section>
    <section v-else-if="subscription" class="section-block empty-inline">
      <div><h2>{{ t('subscription.noCombo') }}</h2><p>{{ t('subscription.noComboHint') }}</p><UButton to="/catalog" :label="t('nav.explore')" color="neutral" variant="outline" /></div>
    </section>
  </div>
</template>

<style scoped>
.page--subscription { display: grid; gap: 1rem; max-width: 760px; }
.page--subscription .page-header { margin-bottom: 0; }
.subscription-page__loading { display: grid; gap: 1rem; }
.subscription-page__hosts { list-style: none; margin: 0; padding: 0; }
.subscription-page__empty, .section-heading small { margin: 0; color: var(--text-muted); font-size: 0.8rem; }
</style>
