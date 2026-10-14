<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeDDNSSettings from './XNodeDDNSSettings.vue'
import XNodeCertificateSettings from './XNodeCertificateSettings.vue'
import XNodeJsonEditor from './XNodeJsonEditor.vue'

const model = defineModel<XNodeSettings>({ required: true })
const { t } = useI18n()
const dns = computed({ get: () => model.value.dns ?? {}, set: value => { model.value = { ...model.value, dns: value } } })
const firewall = computed({ get: () => model.value.firewall ?? { enabled: false, protect: false }, set: value => { model.value = { ...model.value, firewall: value } } })
</script>

<template>
  <div class="xnode-operational">
    <details><summary>{{ t('xnode.ddns') }}</summary><XNodeDDNSSettings v-model="model" /></details>
    <details><summary>{{ t('xnode.certificates') }}</summary><XNodeCertificateSettings v-model="model" /></details>
    <details><summary>{{ t('xnode.dns') }}</summary><div class="grid gap-4 py-4"><USelect v-model="dns.mode" :items="[{ label: 'Xray', value: 'xray' }, { label: 'SmartDNS', value: 'smartdns' }]" class="w-full" /><USwitch v-model="dns.redirect" :label="t('xnode.dnsRedirect')" /><XNodeJsonEditor :model-value="dns.upstreams ?? []" :label="t('xnode.dnsUpstreams')" :rows="5" @update:model-value="value => { dns.upstreams = value as string[] }" /></div></details>
    <details><summary>{{ t('xnode.firewall') }}</summary><div class="grid gap-4 py-4"><USwitch v-model="firewall.enabled" :label="t('common.enabled')" /><USwitch v-model="firewall.protect" :label="t('xnode.probeProtection')" /><XNodeJsonEditor v-model="firewall" :label="t('xnode.firewallPolicy')" :rows="12" /></div></details>
  </div>
</template>

<style scoped>
.xnode-operational { display: grid; gap: 12px; }
summary { cursor: pointer; font-weight: 600; padding-block: 8px; }
</style>
