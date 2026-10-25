<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { XNodeEndpoint, XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeDDNSSettings from './XNodeDDNSSettings.vue'
import XNodeCertificateSettings from './XNodeCertificateSettings.vue'
import XNodeFirewallRules from './XNodeFirewallRules.vue'
import XNodeFirewallFields from './XNodeFirewallFields.vue'
import XNodeDNSSettings from './XNodeDNSSettings.vue'
import XNodeBandwidthSettings from './XNodeBandwidthSettings.vue'

const model = defineModel<XNodeSettings>({ required: true })
withDefaults(defineProps<{ inbounds?: string[]; hosts?: readonly XNodeEndpoint[] }>(), { inbounds: () => [], hosts: () => [] })
const emit = defineEmits<{ valid: [value: boolean] }>()
const firewallValid = shallowRef(true)
const ddnsValid = shallowRef(true)
watch([firewallValid, ddnsValid], ([firewall, ddns]) => emit('valid', firewall && ddns), { immediate: true })
const { t } = useI18n()
const dns = computed({ get: () => model.value.dns ?? {}, set: value => { model.value = { ...model.value, dns: value } } })
const firewall = computed({ get: () => model.value.firewall ?? { enabled: false, protect: false }, set: value => { model.value = { ...model.value, firewall: value } } })
</script>

<template>
  <div class="xnode-operational">
    <details><summary>{{ t('xnode.ddns') }}</summary><XNodeDDNSSettings v-model="model" :hosts="hosts" @valid="ddnsValid = $event" /></details>
    <details><summary>{{ t('xnode.certificates') }}</summary><XNodeCertificateSettings v-model="model" :inbounds="inbounds" /></details>
    <details><summary>{{ t('xnode.dns') }}</summary><XNodeDNSSettings v-model="dns" /></details>
    <details><summary>{{ t('nodeForms.bandwidth') }}</summary><XNodeBandwidthSettings v-model="model" /></details>
    <details><summary>{{ t('xnode.firewall') }}</summary><div class="grid gap-4 py-4"><XNodeFirewallRules v-model="firewall" /><XNodeFirewallFields v-model="firewall" @valid="firewallValid = $event" /></div></details>
  </div>
</template>

<style scoped>
.xnode-operational { display: grid; gap: 12px; }
summary { cursor: pointer; font-weight: 600; padding-block: 8px; }
</style>
