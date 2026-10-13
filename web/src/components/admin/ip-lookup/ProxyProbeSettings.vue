<script setup lang="ts">
import { computed } from 'vue'
import type { IPLookupAdminSettings, IPLookupProxySourcesStatus } from '@/api/ipLookup'
import { createUuid } from '@/utils/browserCompatibility'
import ProxySourceTestControl from './ProxySourceTestControl.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { ipLookupErrorText } from '@/components/ip-lookup/presentation'
import { useI18n } from '@/i18n'
const settings = defineModel<IPLookupAdminSettings>({ required: true })
const props = defineProps<{ disabled?: boolean; savedConfig: string; sourceStatus: IPLookupProxySourcesStatus | null; starting: string[]; errorCode: string }>()
defineEmits<{ refresh: [id: string] }>()
const statuses = computed(() => new Map(props.sourceStatus?.sources.map(source => [source.id, source]) ?? []))
const { t } = useI18n()
const categories = computed(() => (['proxy_pool', 'proxy_database', 'abuse'] as const).map(value => ({ value, label: t(`ipLookup.sourceCategories.${value}`) })))
const resolvers = computed({ get: () => (settings.value.dnsResolvers ?? []).join('\n'), set: value => { settings.value.dnsResolvers = value.split(/\s+/).filter(Boolean) } })
const ports = computed({ get: () => (settings.value.probePorts ?? [443, 4430, 8443]).join(', '), set: value => { settings.value.probePorts = value.split(/[\s,]+/).filter(Boolean).map(Number) } })
function addSource() { (settings.value.proxySources ??= []).push({ id: `source_${createUuid()}`, name: '', url: '', enabled: true, category: 'proxy_pool' }) }
function removeSource(id: string) { settings.value.proxySources = settings.value.proxySources?.filter(source => source.id !== id) }
</script>

<template>
  <section class="proxy-settings">
    <h2>{{ $t('ipLookup.proxySettings') }}</h2><p>{{ $t('ipLookup.proxySettingsHelp') }}</p>
    <p v-if="sourceStatus">{{ $t('ipLookup.inventoryTotal', { count: sourceStatus.uniqueIPs }) }}</p>
    <InlineNotice v-if="errorCode" tone="warning">{{ ipLookupErrorText(errorCode) }}</InlineNotice>
    <UTable class="admin-table" :data="settings.proxySources" :columns="[{ id: 'enabled', header: t('ipLookup.sourceEnabled') }, { id: 'name', header: t('ipLookup.sourceName') }, { id: 'category', header: t('ipLookup.sourceCategory') }, { id: 'url', header: t('ipLookup.sourceURL') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }]">
      <template #enabled-cell="{ row }"><USwitch v-model="row.original.enabled" :aria-label="t('ipLookup.sourceEnabled')" :disabled="disabled" /></template>
      <template #name-cell="{ row }"><UFormField :name="`proxySources.${row.index}.name`"><UInput v-model="row.original.name" :aria-label="t('ipLookup.sourceName')" :disabled="disabled" :maxlength="80" /></UFormField></template>
      <template #category-cell="{ row }"><UFormField :name="`proxySources.${row.index}.category`"><USelect v-model="row.original.category" :aria-label="t('ipLookup.sourceCategory')" :items="categories" :disabled="disabled" /></UFormField></template>
      <template #url-cell="{ row }"><UFormField :name="`proxySources.${row.index}.url`"><UInput v-model="row.original.url" type="url" :aria-label="t('ipLookup.sourceURL')" :disabled="disabled" /></UFormField></template>
      <template #status-cell="{ row }"><ProxySourceTestControl :source="statuses.get(row.original.id)" :starting="starting.includes(row.original.id)" :dirty="savedConfig !== JSON.stringify({ sources: settings.proxySources, resolvers: settings.dnsResolvers })" :disabled="disabled" @refresh="$emit('refresh', row.original.id)" /></template>
      <template #actions-cell="{ row }"><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('ipLookup.removeSource')" :disabled="disabled" @click="removeSource(row.original.id)" /></template>
    </UTable>
    <UButton class="source-add" color="neutral" variant="outline" icon="i-ph-plus" :label="$t('ipLookup.addSource')" :disabled="disabled || (settings.proxySources?.length ?? 0) >= 64" @click="addSource" />
    <div class="probe-settings">
      <UFormField name="dnsResolvers" :label="$t('ipLookup.dnsResolvers')" :help="$t('ipLookup.dnsResolversHelp')"><UTextarea v-model="resolvers" class="w-full" :rows="3" :disabled="disabled" /></UFormField>
      <UFormField name="probePorts" :label="$t('ipLookup.probePorts')" :help="$t('ipLookup.probePortsHelp')"><UInput v-model="ports" class="w-full" :disabled="disabled" /></UFormField>
      <UFormField name="refreshQuotaCost" :label="$t('ipLookup.refreshQuotaCost')" :help="$t('ipLookup.refreshQuotaHelp')"><UInputNumber v-model="settings.refreshQuotaCost" class="w-full" :min="0" :max="1000000" :step="1" :disabled="disabled" /></UFormField>
    </div>
  </section>
</template>

<style scoped>
.proxy-settings { display: grid; gap: 0.7rem; }
.proxy-settings h2 { font-size: 1rem; margin: 0; }
.proxy-settings > p { font-size: 0.8rem; color: var(--text-muted); margin: 0; }
.source-row { display: grid; grid-template-columns: minmax(0, 1fr) 44px; gap: 0.5rem; padding: 0.65rem 0; border-bottom: 1px solid var(--line); }
.source-enabled { grid-column: 1 / -1; }
.source-name, .source-category, .source-url { grid-column: 1; min-width: 0; }
.source-remove { grid-column: 2; grid-row: 2 / 5; align-self: center; }
.source-add { justify-self: start; }
.probe-settings { display: grid; gap: 1rem; }
@media (min-width: 900px) { .source-row { grid-template-columns: 8.5rem minmax(0, 1fr) 9rem minmax(0, 2fr) auto 44px; align-items: center; }.source-enabled, .source-name, .source-category, .source-url, .source-remove { grid-column: auto; grid-row: auto; }.source-remove { grid-column: 6; grid-row: 1; }.probe-settings { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
