<script setup lang="ts">
import { computed, shallowRef } from 'vue'
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

const { settings, combos, loading, loaded, busy, errorCode, saved, save, load } = useAdminIPLookup()
async function saveSettings() { await save() }
const { t } = useI18n()
const blocked = computed(() => busy.value || loading.value || !loaded.value)
const workspace = shallowRef('general')
const tabs = computed(() => [{ value: 'general', label: t('adminWorkspace.configuration') }, { value: 'providers', label: t('ipLookup.providerHeading') }, { value: 'quotas', label: t('ipLookup.quotaHeading') }])
const invalidConfig = computed(() => t('ipLookup.errors.IP_LOOKUP_INVALID_CONFIG'))
const checkOrder = computed({
  get: () => settings.providers.map(provider => provider.id),
  set: ids => { settings.providers = ids.flatMap(id => settings.providers.filter(provider => provider.id === id)) },
})
const schema = computed(() => z.object({
  lookupFeeTxb: z.string().refine(value => !value || /^\d+(\.\d{1,2})?$/.test(value), t('ipLookup.errors.IP_LOOKUP_INVALID_CONFIG')),
  refreshFeeTxb: z.string().refine(value => !value || /^\d+(\.\d{1,2})?$/.test(value), t('ipLookup.errors.IP_LOOKUP_INVALID_CONFIG')),
  refreshQuotaCost: z.number({ error: invalidConfig.value }).int(invalidConfig.value).min(0, invalidConfig.value).max(1000000, invalidConfig.value),
  retry: z.object({
    enabled: z.boolean(),
    maxAttempts: z.number().int().min(1).max(10, t('ipLookup.errors.IP_LOOKUP_INVALID_CONFIG')),
    delaySeconds: z.number().int().min(1).max(86400, t('ipLookup.errors.IP_LOOKUP_INVALID_CONFIG')),
  }),
}).passthrough())
</script>

<template>
  <section class="admin-panel admin-ip-lookup">
    <div class="admin-panel__heading"><div><h2>{{ t('ipLookup.adminTitle') }}</h2><p>{{ t('ipLookup.adminSubtitle') }}</p></div><UButton type="submit" form="ip-lookup-settings" icon="i-ph-floppy-disk" :loading="busy" :disabled="blocked" :label="t('common.save')" /></div>
    <UTabs v-model="workspace" :items="tabs" :content="false" variant="link" class="admin-workspace-tabs" />
    <SkeletonBlock v-if="loading" height="14rem" />
    <template v-else>
      <InlineNotice v-if="errorCode" tone="warning">{{ ipLookupErrorText(errorCode) }}</InlineNotice>
      <InlineNotice v-if="saved" tone="success">{{ t('ipLookup.saved') }}</InlineNotice>
      <UForm id="ip-lookup-settings" :state="settings" :schema="schema" class="lookup-settings" @submit="saveSettings">
        <fieldset class="lookup-settings__fields" :disabled="blocked">
          <div v-show="workspace === 'general'" class="lookup-general">
            <SwitchField id="ip-lookup-enabled" v-model="settings.enabled" :label="t('ipLookup.enable')" :help="t('ipLookup.enableHelp')" />
            <div class="lookup-fees"><TxbAmountField id="lookup-fee" v-model="settings.lookupFeeTxb" :label="t('ipLookup.lookupFee')" min-minor="0" /><TxbAmountField id="refresh-fee" v-model="settings.refreshFeeTxb" :label="t('ipLookup.refreshFee')" min-minor="0" /></div>
            <div class="lookup-orders"><ProviderOrder v-model="checkOrder" :title="t('ipLookup.checkOrder')" :help="t('ipLookup.checkOrderHelp')" :disabled="blocked" /><ProviderOrder v-model="settings.geolocationOrder" :title="t('ipLookup.geolocationOrder')" :help="t('ipLookup.geolocationOrderHelp')" :disabled="blocked" /></div>
            <UFormField name="refreshQuotaCost" :label="t('ipLookup.refreshQuotaCost')" :help="t('ipLookup.refreshQuotaHelp')"><UInputNumber v-model="settings.refreshQuotaCost" :min="0" :max="1000000" :step="1" :disabled="blocked" /></UFormField>
            <RetrySettings v-if="settings.retry" v-model="settings.retry" :disabled="blocked" />
          </div>
          <ProviderSettings v-show="workspace === 'providers'" v-model="settings" />
          <ComboQuotas v-show="workspace === 'quotas'" v-model="settings.comboQuotas" :combos="combos" />
        </fieldset>
      </UForm>
      <UButton v-if="errorCode && !loaded" color="neutral" variant="outline" :label="t('common.tryAgain')" @click="load" />
    </template>
  </section>
</template>
<style scoped>
.lookup-settings, .lookup-settings__fields, .lookup-general { display: grid; gap: 1.5rem; min-width: 0; }
.lookup-settings__fields { border: 0; margin: 0; padding: 0; }
.lookup-fees, .lookup-orders { display: grid; gap: 1.25rem; }
@media (min-width: 1024px) { .lookup-fees, .lookup-orders { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
