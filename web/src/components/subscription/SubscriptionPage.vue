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
import SquadUptime from './SquadUptime.vue'
import { useSubscriptionData } from './useSubscriptionData'
import SubscriptionFormats from './SubscriptionFormats.vue'

const { value: subscription, loading, error, refresh } = useSubscriptionData(subscriptionApi.subscription)
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
    <template v-else-if="subscription?.activeCombo">
      <SubscriptionPanel :subscription-url="subscription.subscriptionUrl" :revoking="busy" :revoke-blocked="blocksMutations" :open-revoke="revokeRequested" show-desktop-actions @revoke="revoke" @revoke-request-consumed="consumeRevoke" />
      <SubscriptionFormats :key="subscription.subscriptionUrl ?? ''" />
      <UButton to="/settings/diy" color="neutral" variant="outline" icon="i-ph-sliders-horizontal-bold" :label="t('diy.title')" class="justify-self-start" />
      <section class="section-block">
        <div class="section-heading"><h2>{{ t('subscription.hosts') }}</h2></div>
        <ul v-if="subscription.hosts.length" class="subscription-page__hosts"><HostLinkRow v-for="host in subscription.hosts" :key="host.uuid" :host="host" /></ul>
        <p v-else class="subscription-page__empty">{{ t('subscription.noHosts') }}</p>
      </section>
      <section v-if="subscription.summary.monitoringEnabled" class="section-block">
        <div class="section-heading"><h2>{{ t('subscription.squads') }}</h2><small>{{ t('uptime.window') }}</small></div>
        <p v-if="subscription.summary.errorCode" class="subscription-page__empty">{{ t('uptime.notAvailable') }}</p>
        <SquadUptime v-for="squad in subscription.squads" :key="squad.uuid" :squad="squad" :hosts="subscription.hosts" />
        <p v-if="!subscription.squads.length" class="subscription-page__empty">{{ t('subscription.noSquads') }}</p>
      </section>
    </template>
    <section v-else-if="subscription" class="section-block empty-inline">
      <div><h2>{{ t('subscription.noCombo') }}</h2><p>{{ t('subscription.noComboHint') }}</p><UButton to="/catalog" :label="t('nav.explore')" color="neutral" variant="outline" /></div>
    </section>
  </div>
</template>

<style scoped>
.page--subscription { display: grid; gap: 1rem; max-width: 760px; }
.page--subscription .page-header { margin-bottom: 0; }
.page--subscription .page-header h1 { font-size: 1.5rem; line-height: 1.2; }
.subscription-page__loading { display: grid; gap: 1rem; }
.subscription-page__hosts { list-style: none; margin: 0; padding: 0; }
.subscription-page__empty, .section-heading small { margin: 0; color: var(--text-muted); font-size: 0.8rem; }
</style>
