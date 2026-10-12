<script setup lang="ts">
import { computed } from 'vue'
import type { IPLookupAdminSettings, IPLookupProxySourcesStatus } from '@/api/ipLookup'
import { createUuid } from '@/utils/browserCompatibility'
import SwitchField from '@/components/common/SwitchField.vue'
import ProxySourceTestControl from './ProxySourceTestControl.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { ipLookupErrorText } from '@/components/ip-lookup/presentation'
const settings = defineModel<IPLookupAdminSettings>({ required: true })
const props = defineProps<{ disabled?: boolean; savedConfig: string; sourceStatus: IPLookupProxySourcesStatus | null; starting: string[]; errorCode: string }>()
defineEmits<{ refresh: [id: string] }>()
const statuses = computed(() => new Map(props.sourceStatus?.sources.map(source => [source.id, source]) ?? []))
const resolvers = computed({ get: () => (settings.value.dnsResolvers ?? []).join('\n'), set: value => { settings.value.dnsResolvers = value.split(/\s+/).filter(Boolean) } })
const ports = computed({ get: () => (settings.value.probePorts ?? [443, 4430, 8443]).join(', '), set: value => { settings.value.probePorts = value.split(/[\s,]+/).filter(Boolean).map(Number) } })
function addSource() { (settings.value.proxySources ??= []).push({ id: `source_${createUuid()}`, name: '', url: '', enabled: true }) }
function removeSource(id: string) { settings.value.proxySources = settings.value.proxySources?.filter(source => source.id !== id) }
</script>

<template>
  <section class="proxy-settings">
    <h2>{{ $t('ipLookup.proxySettings') }}</h2><p>{{ $t('ipLookup.proxySettingsHelp') }}</p>
    <p v-if="sourceStatus">{{ $t('ipLookup.inventoryTotal', { count: sourceStatus.uniqueIPs }) }}</p>
    <InlineNotice v-if="errorCode" tone="warning">{{ ipLookupErrorText(errorCode) }}</InlineNotice>
    <div v-for="(source, index) in settings.proxySources" :key="source.id" class="source-row">
      <SwitchField :id="`proxy-${source.id}`" v-model="source.enabled" :label="$t('ipLookup.sourceEnabled')" :disabled="disabled" />
      <UFormField :name="`proxySources.${index}.name`" :label="$t('ipLookup.sourceName')"><UInput v-model="source.name" class="w-full" :disabled="disabled" :maxlength="80" /></UFormField>
      <UFormField :name="`proxySources.${index}.url`" :label="$t('ipLookup.sourceURL')"><UInput v-model="source.url" class="w-full" type="url" :disabled="disabled" /></UFormField>
      <ProxySourceTestControl :source="statuses.get(source.id)" :starting="starting.includes(source.id)" :dirty="savedConfig !== JSON.stringify({ sources: settings.proxySources, resolvers: settings.dnsResolvers })" :disabled="disabled" @refresh="$emit('refresh', source.id)" />
      <UButton color="neutral" variant="ghost" icon="i-ph-trash" :aria-label="$t('ipLookup.removeSource')" :disabled="disabled" @click="removeSource(source.id)" />
    </div>
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
.source-row > :first-child { grid-column: 1 / -1; }
.source-row > :nth-child(3) { grid-column: 1; }
.source-row > :last-child { grid-column: 2; grid-row: 2 / 4; align-self: center; }
.source-row > :nth-child(4) { grid-column: 1 / -1; }
.source-add { justify-self: start; }
.probe-settings { display: grid; gap: 1rem; }
@media (min-width: 900px) { .source-row { grid-template-columns: 8.5rem minmax(0, 1fr) minmax(0, 2fr) auto 44px; align-items: center; }.source-row > :first-child, .source-row > :nth-child(3), .source-row > :nth-child(4), .source-row > :last-child { grid-column: auto; grid-row: auto; }.source-row > :last-child { grid-column: 5; grid-row: 1; }.probe-settings { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
