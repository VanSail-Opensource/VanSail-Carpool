<script setup lang="ts">
import { computed } from 'vue'
import { z } from 'zod'
import { useI18n } from '@/i18n'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { ipLookupErrorText } from '@/components/ip-lookup/presentation'
import ProviderSettings from './ProviderSettings.vue'
import ComboQuotas from './ComboQuotas.vue'
import ProviderOrder from './ProviderOrder.vue'
import RetrySettings from './RetrySettings.vue'
import { useAdminIPLookup } from './useAdminIPLookup'

const { settings, combos, loading, busy, errorCode, saved, save, load } = useAdminIPLookup()
const { t } = useI18n()
const checkOrder = computed({
  get: () => settings.providers.map(provider => provider.id),
  set: ids => { settings.providers = ids.flatMap(id => settings.providers.filter(provider => provider.id === id)) },
})
const schema = computed(() => z.object({
  lookupFeeTxb: z.string().refine(value => !value || /^\d+(\.\d{1,2})?$/.test(value), t('ipLookup.errors.IP_LOOKUP_INVALID_CONFIG')),
  refreshFeeTxb: z.string().refine(value => !value || /^\d+(\.\d{1,2})?$/.test(value), t('ipLookup.errors.IP_LOOKUP_INVALID_CONFIG')),
  retry: z.object({
    enabled: z.boolean(),
    maxAttempts: z.number().int().min(1).max(10, t('ipLookup.errors.IP_LOOKUP_INVALID_CONFIG')),
    delaySeconds: z.number().int().min(1).max(86400, t('ipLookup.errors.IP_LOOKUP_INVALID_CONFIG')),
  }),
}).passthrough())
</script>

<template>
  <main class="page admin-ip-lookup">
    <header class="page-header"><div><p class="eyebrow">{{ $t('nav.admin') }}</p><h1>{{ $t('ipLookup.adminTitle') }}</h1><p>{{ $t('ipLookup.adminSubtitle') }}</p></div></header>
    <SkeletonBlock v-if="loading" height="14rem" />
    <template v-else>
      <InlineNotice v-if="errorCode" tone="warning">{{ ipLookupErrorText(errorCode) }}</InlineNotice>
      <InlineNotice v-if="saved" tone="success">{{ $t('ipLookup.saved') }}</InlineNotice>
      <UForm :state="settings" :schema="schema" class="lookup-settings" @submit="save">
        <SwitchField id="ip-lookup-enabled" v-model="settings.enabled" :label="$t('ipLookup.enable')" :help="$t('ipLookup.enableHelp')" />
        <div class="lookup-fees">
          <TxbAmountField id="lookup-fee" v-model="settings.lookupFeeTxb" :label="$t('ipLookup.lookupFee')" min-minor="0" />
          <TxbAmountField id="refresh-fee" v-model="settings.refreshFeeTxb" :label="$t('ipLookup.refreshFee')" min-minor="0" />
        </div>
        <div class="lookup-orders">
          <ProviderOrder v-model="checkOrder" :title="$t('ipLookup.checkOrder')" :help="$t('ipLookup.checkOrderHelp')" :disabled="busy" />
          <ProviderOrder v-model="settings.geolocationOrder" :title="$t('ipLookup.geolocationOrder')" :help="$t('ipLookup.geolocationOrderHelp')" :disabled="busy" />
        </div>
        <ProviderSettings v-model="settings" />
        <RetrySettings v-if="settings.retry" v-model="settings.retry" :disabled="busy" />
        <ComboQuotas v-model="settings.comboQuotas" :combos="combos" />
        <UButton type="submit" :loading="busy" :disabled="busy" :label="$t('common.save')" class="lookup-save" />
      </UForm>
      <UButton v-if="errorCode && !settings.providers.length" color="neutral" variant="outline" :label="$t('common.tryAgain')" @click="load" />
    </template>
  </main>
</template>

<style scoped>
.admin-ip-lookup { max-width: 1080px; }
.lookup-settings { display: grid; gap: 1.5rem; }
.lookup-fees { display: grid; gap: 1rem; }
.lookup-orders { display: grid; gap: 1.25rem; }
.lookup-save { justify-self: start; min-height: 44px; }
@media (min-width: 900px) { .lookup-fees, .lookup-orders { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
