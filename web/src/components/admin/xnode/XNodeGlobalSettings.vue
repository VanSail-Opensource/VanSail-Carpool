<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { xnodeApi, type XNodeGlobal } from '@/api/xnode'
import { localizedError, useI18n } from '@/i18n'
import XNodeJsonEditor from './XNodeJsonEditor.vue'

const { t } = useI18n()
const draft = shallowRef<XNodeGlobal | null>(null)
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const validProviders = shallowRef(true)
async function load(): Promise<void> {
  loading.value = true; error.value = null
  try { draft.value = await xnodeApi.global() }
  catch (caught) { error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { loading.value = false }
}
async function save(): Promise<void> {
  if (!draft.value || !validProviders.value || busy.value) return
  busy.value = true; error.value = null
  try { draft.value = await xnodeApi.saveGlobal(draft.value) }
  catch (caught) { error.value = localizedError(caught, 'xnode.saveFailed') }
  finally { busy.value = false }
}
onMounted(() => void load())
</script>

<template>
  <UProgress v-if="loading" />
  <UAlert v-if="error" color="warning" variant="soft" :title="error" />
  <form v-if="draft" class="global-settings" @submit.prevent="save">
    <div class="global-fields">
      <UFormField :label="t('xnode.connectionLimit')" :description="t('xnode.connectionLimitHint')"><UInput v-model.number="draft.connectionLimit" type="number" min="0" max="1000000" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.blockedPorts')" :description="t('xnode.blockedPortsHint')"><UInput v-model="draft.blockedPorts" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.geoIpUrl')"><UInput v-model="draft.geoIpUrl" type="url" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.geoSiteUrl')"><UInput v-model="draft.geoSiteUrl" type="url" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.geoIpHash')"><UInput v-model="draft.geoIpHash" class="w-full font-mono" /></UFormField>
      <UFormField :label="t('xnode.geoSiteHash')"><UInput v-model="draft.geoSiteHash" class="w-full font-mono" /></UFormField>
      <UFormField :label="t('xnode.geodataCron')"><UInput v-model="draft.geodataCron" class="w-full" /></UFormField>
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
    <details><summary>{{ t('xnode.dnsProviders') }}</summary><p class="text-muted text-xs py-2">{{ t('xnode.providerSecretsHint') }}</p><XNodeJsonEditor v-model="draft.providers" :label="t('xnode.dnsProviders')" @valid="validProviders = $event" /></details>
    <div class="flex justify-end"><UButton type="submit" :loading="busy" :disabled="!validProviders" :label="t('common.save')" /></div>
  </form>
</template>

<style scoped>
.global-settings { display: grid; gap: 20px; }
.global-fields { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 16px 24px; }
summary { cursor: pointer; font-weight: 600; }
@media(max-width:600px) { .global-fields { grid-template-columns: minmax(0,1fr); } }
</style>
