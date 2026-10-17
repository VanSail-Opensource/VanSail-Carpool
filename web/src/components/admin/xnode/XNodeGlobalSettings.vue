<script setup lang="ts">
import { shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import XNodeGlobalPolicies from './XNodeGlobalPolicies.vue'
import XNodeDNSProviders from './XNodeDNSProviders.vue'
import { useXNodeGlobalSettings } from './useXNodeGlobalSettings'
const { t } = useI18n()
const { draft, loading, busy, error, canSave, load, save } = useXNodeGlobalSettings()
const validPolicies = shallowRef(true)
</script>

<template>
  <UProgress v-if="loading" />
  <UAlert v-if="error" color="warning" variant="soft" :title="error" />
  <UButton v-if="!loading && !canSave && !busy" class="justify-self-start" variant="outline" color="neutral" :label="t('common.tryAgain')" @click="load" />
  <form v-if="draft" class="global-settings" @submit.prevent="save">
    <fieldset :disabled="!canSave" class="contents">
      <div class="global-fields">
        <UFormField :label="t('firewall.price')" :description="t('firewall.priceHint')"><UInput v-model="draft.firewallUnbanPrice" inputmode="decimal" class="w-full" /></UFormField>
        <UFormField :label="t('firewall.hours')"><UInput v-model.number="draft.firewallExemptionHours" type="number" min="0" max="8760" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.connectionLimit')" :description="t('xnode.connectionLimitHint')"><UInput v-model.number="draft.connectionLimit" type="number" min="0" max="1000000" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.blockedPorts')" :description="t('xnode.blockedPortsHint')"><UInput v-model="draft.blockedPorts" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.geoIpUrl')"><UInput v-model="draft.geoIpUrl" type="url" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.geoSiteUrl')"><UInput v-model="draft.geoSiteUrl" type="url" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.geoIpHash')"><UInput v-model="draft.geoIpHash" class="w-full font-mono" /></UFormField>
        <UFormField :label="t('xnode.geoSiteHash')"><UInput v-model="draft.geoSiteHash" class="w-full font-mono" /></UFormField>
        <UFormField :label="t('xnode.torrentBanSeconds')"><UInput v-model.number="draft.torrent.ban_seconds" type="number" min="1" max="604800" class="w-full" /></UFormField>
        <UFormField><USwitch v-model="draft.torrent.enabled" :label="t('xnode.torrentBlocker')" /></UFormField>
        <UFormField><USwitch v-model="draft.telemetry.include_active_ips" :label="t('xnode.includeActiveIPs')" /></UFormField>
        <UFormField :label="t('xnode.sampleSeconds')"><UInput v-model.number="draft.telemetry.sample_seconds" type="number" min="1" max="3600" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.reportSeconds')"><UInput v-model.number="draft.telemetry.report_seconds" type="number" min="1" max="3600" class="w-full" /></UFormField>
        <UFormField><USwitch v-model="draft.updates.enabled" :label="t('xnode.automaticUpdates')" /></UFormField>
        <UFormField :label="t('xnode.updateVersion')"><UInput v-model="draft.updates.version" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.speedtestDownloadUrl')"><UInput v-model="draft.speedtestDownloadUrl" type="url" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.speedtestUploadUrl')"><UInput v-model="draft.speedtestUploadUrl" type="url" class="w-full" /></UFormField>
      </div>
      <XNodeGlobalPolicies v-model="draft" @valid="validPolicies = $event" />
      <details><summary>{{ t('xnode.dnsProviders') }}</summary><XNodeDNSProviders v-model="draft.providers" /></details>
      <div class="flex justify-end"><UButton type="submit" :loading="busy" :disabled="!validPolicies || !canSave" :label="t('common.save')" /></div>
    </fieldset>
  </form>
</template>

<style scoped>
.global-settings { display: grid; gap: 20px; }
.global-fields { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 16px 24px; }
summary { cursor: pointer; font-weight: 600; }
@media(max-width:600px) { .global-fields { grid-template-columns: minmax(0,1fr); } }
</style>
