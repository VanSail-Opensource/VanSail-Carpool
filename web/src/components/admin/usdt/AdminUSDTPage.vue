<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { USDTNetworkConfig, USDTNetworkID, USDTQuotaGroup } from '@/api/usdt'
import InlineNotice from '@/components/common/InlineNotice.vue'
import AdminSectionState from '@/components/admin/AdminSectionState.vue'
import { paymentChannelLabel } from '@/components/billing/paymentOptions'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import { configSchema, networkRails, newQuotaGroup } from './configForm'
import { useUSDTAdmin } from './useUSDTAdmin'
import USDTGeneralSettings from './USDTGeneralSettings.vue'
import USDTNetworkSettings from './USDTNetworkSettings.vue'
import USDTQuotaSettings from './USDTQuotaSettings.vue'
import USDTReview from './USDTReview.vue'
const { t } = useI18n()
const { draft, status, review, loading, busy, checking, resolving, error, saved, probe, load, refresh, save, check, loadReview, resolve } = useUSDTAdmin()
const selected = shallowRef<USDTNetworkID>('tron')
const schema = computed(() => configSchema())
const networkItems = computed(() => draft.value?.networks.map(network => ({ value: network.network, label: paymentChannelLabel(networkRails[network.network], network.network) })) ?? [])
const index = computed(() => draft.value?.networks.findIndex(network => network.network === selected.value) ?? -1)
const network = computed<USDTNetworkConfig | null>({
  get: () => draft.value?.networks[index.value] ?? null,
  set: value => { if (draft.value && value) draft.value = { ...draft.value, networks: draft.value.networks.map((item, position) => position === index.value ? value : item) } },
})
const currentStatus = computed(() => status.value?.networks.find(item => item.network === selected.value))
const state = computed(() => currentStatus.value?.state ?? 'disabled')
const progress = computed(() => {
  const value = currentStatus.value?.cursor
  if (!value) return t('common.notAvailable')
  try { const cursor = JSON.parse(value) as Record<string, unknown>; return String(cursor.version ?? cursor.baseline ?? cursor.completed ?? cursor.end ?? value) }
  catch { return value }
})
function setGroups(groups: USDTQuotaGroup[]): void { if (draft.value) draft.value = { ...draft.value, groups } }
function ensureGroup(): void { if (draft.value && !draft.value.groups.length) setGroups([newQuotaGroup('rpc-default')]) }
function renameGroup(before: string, after: string): void {
  if (draft.value) draft.value = { ...draft.value, networks: draft.value.networks.map(item => ({ ...item, endpoints: item.endpoints.map(endpoint => endpoint.group === before ? { ...endpoint, group: after } : endpoint) })) }
}
</script>

<template>
  <section class="admin-panel usdt-admin">
    <div class="admin-panel__heading"><div><h2>{{ t('adminUsdt.title') }}</h2><p>{{ t('adminUsdt.copy') }}</p></div><UButton icon="i-ph-floppy-disk" type="submit" form="usdt-settings" :label="t('common.save')" :loading="busy" :disabled="busy || loading || !draft" /></div>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <InlineNotice v-if="saved" tone="success">{{ t('adminUsdt.saved') }}</InlineNotice>
    <AdminSectionState :loading="loading" :error="draft ? null : error" @retry="load">
      <UForm v-if="draft" id="usdt-settings" :schema="schema" :state="draft" @submit="save">
        <fieldset :disabled="busy" class="usdt-form">
          <USDTGeneralSettings v-model="draft" />
          <section class="usdt-network-section">
            <div class="usdt-toolbar">
              <UFormField :label="t('adminUsdt.network')"><USelect v-model="selected" :items="networkItems" /></UFormField>
              <UBadge :color="state === 'ready' ? 'success' : state === 'error' ? 'error' : 'neutral'" variant="soft" :label="t(`adminUsdt.states.${state}`)" />
              <UButton color="neutral" variant="ghost" icon="i-ph-plugs-connected" :label="t('adminUsdt.check')" :loading="checking === selected" :disabled="!!checking || !network?.endpoints.length" @click="check(selected)" />
            </div>
            <div class="usdt-status" aria-live="polite"><span>{{ t(`adminUsdt.connections.${currentStatus?.connection || 'none'}`) }}</span><span>{{ t('adminUsdt.progress') }} <code>{{ progress }}</code></span><time v-if="currentStatus?.updatedAt">{{ formatDateTime(currentStatus.updatedAt) }}</time><UButton color="neutral" variant="ghost" icon="i-ph-arrows-clockwise" :aria-label="t('common.refresh')" @click="refresh" /></div>
            <InlineNotice v-if="currentStatus?.errorCode" tone="warning">{{ t(`errors.api.${currentStatus.errorCode}`) }}</InlineNotice>
            <InlineNotice v-if="probe" tone="info">{{ probe }}</InlineNotice>
            <USDTNetworkSettings v-if="network" v-model="network" :groups="draft.groups" :index="index" @ensure-group="ensureGroup" />
          </section>
          <USDTQuotaSettings :model-value="draft.groups" :status="status" @update:model-value="setGroups" @rename="renameGroup" />
        </fieldset>
      </UForm>
      <USDTReview :page="review" :resolving="resolving" @refresh="loadReview()" @more="loadReview(true)" @resolve="resolve" />
    </AdminSectionState>
  </section>
</template>

<style scoped>
.usdt-admin { max-width: 64rem; }
.usdt-form { border: 0; margin: 0; padding: 1rem; min-width: 0; display: grid; gap: 1.25rem; }
.usdt-admin :deep(.usdt-toolbar) { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: .5rem; }
.usdt-admin :deep(.usdt-form-grid), .usdt-admin :deep(.usdt-network), .usdt-admin :deep(.usdt-general) { display: grid; gap: .8rem; }
.usdt-admin :deep(.usdt-form-grid > *), .usdt-admin :deep(.usdt-toolbar > *) { min-width: 0; }
.usdt-admin :deep(.usdt-form-grid input), .usdt-admin :deep(.usdt-form-grid [data-slot="root"]), .usdt-admin :deep(.usdt-network textarea) { width: 100%; }
.usdt-admin :deep(.usdt-addresses) { width: 100%; }
.usdt-admin :deep(.usdt-endpoint) { border: 0; border-top: 1px solid var(--line); padding: .8rem 0; margin: 0; min-width: 0; display: grid; gap: .75rem; }
.usdt-admin :deep(.usdt-endpoint legend) { font-size: .8rem; color: var(--text-muted); padding: 0 .4rem 0 0; }
.usdt-network-section { display: grid; gap: .8rem; border-top: 1px solid var(--line); padding-top: 1rem; }
.usdt-status { display: flex; align-items: center; flex-wrap: wrap; gap: .7rem; color: var(--text-muted); font-size: .75rem; }
.usdt-status code { overflow-wrap: anywhere; }
.usdt-admin :deep(.usdt-quotas summary) { cursor: pointer; font-size: .9rem; }
.usdt-admin :deep(.usdt-quota) { display: grid; gap: .7rem; padding: .8rem 0; }
.usdt-admin :deep(.usdt-quota-grid) { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .7rem; }
.usdt-admin :deep(.usdt-review) { border-top: 1px solid var(--line); padding: 1rem; display: grid; gap: .7rem; }
.usdt-admin :deep(.usdt-review h3) { margin: 0; font-size: .95rem; }
.usdt-admin :deep(.usdt-review-item) { border-top: 1px solid var(--line); padding-top: .9rem; display: grid; gap: .65rem; }
.usdt-admin :deep(.usdt-evidence) { margin: 0; display: grid; gap: .4rem; font-size: .75rem; }
.usdt-admin :deep(.usdt-evidence dt) { color: var(--text-muted); }
.usdt-admin :deep(.usdt-evidence dd) { margin: 0; overflow-wrap: anywhere; }
@media (min-width: 640px) { .usdt-admin :deep(.usdt-form-grid) { grid-template-columns: repeat(2, minmax(0, 1fr)); } .usdt-admin :deep(.usdt-quota-grid) { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
</style>
