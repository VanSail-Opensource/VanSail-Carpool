<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeDDNSSettings from './XNodeDDNSSettings.vue'
import XNodeCertificateSettings from './XNodeCertificateSettings.vue'
import XNodeFirewallRules from './XNodeFirewallRules.vue'
import XNodeJsonEditor from '@/components/common/JsonDocumentField.vue'

const model = defineModel<XNodeSettings>({ required: true })
defineProps<{ inbounds?: string[] }>()
const emit = defineEmits<{ valid: [value: boolean] }>()
const firewallValid = shallowRef(true)
watch(firewallValid, value => emit('valid', value), { immediate: true })
const { t } = useI18n()
const dns = computed({ get: () => model.value.dns ?? {}, set: value => { model.value = { ...model.value, dns: value } } })
const firewall = computed({ get: () => model.value.firewall ?? { enabled: false, protect: false }, set: value => { model.value = { ...model.value, firewall: value } } })
</script>

<template>
  <div class="xnode-operational">
    <details><summary>{{ t('xnode.ddns') }}</summary><XNodeDDNSSettings v-model="model" /></details>
    <details><summary>{{ t('xnode.certificates') }}</summary><XNodeCertificateSettings v-model="model" :inbounds="inbounds" /></details>
    <details><summary>{{ t('xnode.dns') }}</summary><div class="grid gap-4 py-4"><p class="text-sm text-muted">{{ t('xnode.scopedDnsHint') }}</p><USwitch v-model="dns.redirect" :label="t('xnode.dnsRedirect')" /></div></details>
    <details><summary>{{ t('xnode.firewall') }}</summary><div class="grid gap-4 py-4"><XNodeFirewallRules v-model="firewall" /><XNodeJsonEditor v-model="firewall" :label="t('xnode.firewallPolicy')" :rows="12" @valid="firewallValid = $event" /></div></details>
  </div>
</template>

<style scoped>
.xnode-operational { display: grid; gap: 12px; }
summary { cursor: pointer; font-weight: 600; padding-block: 8px; }
</style>
