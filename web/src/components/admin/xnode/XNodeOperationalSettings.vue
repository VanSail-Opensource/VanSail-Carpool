<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { XNodeEndpoint, XNodeSettings, XNodeMetrics } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeDDNSSettings from './XNodeDDNSSettings.vue'
import XNodeCertificateSettings from './XNodeCertificateSettings.vue'
import XNodeFirewallRules from './XNodeFirewallRules.vue'
import XNodeFirewallFields from './XNodeFirewallFields.vue'
import XNodeDNSSettings from './XNodeDNSSettings.vue'
import { useDDNSDiscovery } from './useDDNSDiscovery'

const model = defineModel<XNodeSettings>({ required: true })
withDefaults(defineProps<{ inbounds?: string[]; hosts?: readonly XNodeEndpoint[]; nodeId?: string; revision?: number; metric?: XNodeMetrics['nodes'][number]; certificatesSaved?: boolean; savedCertificateIds?: string[] }>(), { inbounds: () => [], hosts: () => [], nodeId: '', revision: 0, certificatesSaved: false, savedCertificateIds: () => [], metric: undefined })
const emit = defineEmits<{ valid: [value: boolean] }>()
const firewallValid = shallowRef(true)
const ddnsValid = shallowRef(true)
const certificatesValid = shallowRef(true)
watch([firewallValid, ddnsValid, certificatesValid], ([firewall, ddns, certificates]) => emit('valid', firewall && ddns && certificates), { immediate: true })
const discovery = useDDNSDiscovery()
const { t } = useI18n()
const dns = computed({ get: () => model.value.dns ?? {}, set: value => { model.value = { ...model.value, dns: value } } })
const firewall = computed({ get: () => model.value.firewall ?? { enabled: false, protect: false }, set: value => { model.value = { ...model.value, firewall: value } } })
</script>

<template>
  <div class="xnode-operational">
    <div v-if="discovery.error.value" role="alert"><p>{{ discovery.error.value }}</p><UButton type="button" :label="t('common.tryAgain')" variant="link" @click="discovery.load" /></div>
    <details><summary>{{ t('xnode.ddns') }}</summary><XNodeDDNSSettings v-model="model" :hosts="hosts" :providers="discovery.providers.value" :providers-loading="discovery.loading.value" @valid="ddnsValid = $event" /></details>
    <details><summary>{{ t('xnode.certificates') }}</summary><XNodeCertificateSettings v-model="model" :inbounds="inbounds" :providers="discovery.providers.value" :providers-loading="discovery.loading.value || !!discovery.error.value" :node-id="nodeId" :revision="revision" :metric="metric" :saved="certificatesSaved" :saved-profile-ids="savedCertificateIds" @valid="certificatesValid = $event" /></details>
    <details><summary>{{ t('xnode.dns') }}</summary><XNodeDNSSettings v-model="dns" /></details>
    <details><summary>{{ t('xnode.firewall') }}</summary><div class="grid gap-4 py-4"><XNodeFirewallRules v-model="firewall" /><XNodeFirewallFields v-model="firewall" @valid="firewallValid = $event" /></div></details>
  </div>
</template>

<style scoped>
.xnode-operational { display: grid; gap: 12px; }
summary { cursor: pointer; font-weight: 600; padding-block: 8px; }
</style>
