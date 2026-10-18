<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'

const model = defineModel<XNodeSettings>({ required: true })
defineProps<{ inbounds?: string[] }>()
const { t } = useI18n()
const entries = computed(() => Object.entries(model.value.certificates ?? {}))
const modes = computed(() => ['http', 'dns', 'file', 'content', 'self'].map(value => ({ value, label: t(`xnode.certificateModes.${value}`) })))
const profiles = computed(() => [{ value: '_none', label: t('nodeAdmin.noCertificate') }, ...entries.value.map(([id]) => ({ value: id, label: id }))])
function bind(tag: string, id: string) {
  const bindings = { ...model.value.inboundCertificates }
  if (id === '_none') delete bindings[tag]
  else bindings[tag] = id
  model.value = { ...model.value, inboundCertificates: bindings }
}
function add(): void {
  let suffix = entries.value.length + 1
  while (model.value.certificates?.[`certificate-${suffix}`]) suffix++
  const id = `certificate-${suffix}`
  model.value = { ...model.value, certificates: { ...model.value.certificates, [id]: { provider: '', config: { domain: '', cert_mode: 'http', auto_tls: true } } } }
}
function remove(id: string): void {
  const next = { ...model.value.certificates }; delete next[id]
  const bindings = { ...model.value.inboundCertificates }
  for (const tag of Object.keys(bindings)) if (bindings[tag] === id) delete bindings[tag]
  model.value = { ...model.value, certificates: next, inboundCertificates: bindings }
}
</script>

<template>
  <div class="grid gap-4 py-4">
    <div v-for="[id, profile] in entries" :key="id" class="certificate-profile">
      <div class="certificate-fields">
        <UFormField :label="t('xnode.profileId')"><UInput :model-value="id" readonly class="w-full" /></UFormField>
        <UFormField :label="t('xnode.certificateMode')"><USelect v-model="profile.config.cert_mode" :items="modes" class="w-full" /></UFormField>
        <UFormField v-if="profile.config.cert_mode !== 'file'" :label="t('xnode.domain')"><UInput v-model="profile.config.domain" class="w-full" /></UFormField>
        <UFormField v-if="['http', 'dns'].includes(profile.config.cert_mode)" :label="t('xnode.email')"><UInput v-model="profile.config.email" type="email" class="w-full" /></UFormField>
        <UFormField v-if="profile.config.cert_mode !== 'file'"><USwitch v-model="profile.config.auto_tls" :label="t('nodeAdmin.autoTls')" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'dns'" :label="t('xnode.dnsProvider')"><UInput v-model="profile.provider" class="w-full" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'file'" :label="t('xnode.certificateFile')" required><UInput v-model="profile.config.cert_file" required class="w-full" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'file'" :label="t('xnode.keyFile')" required><UInput v-model="profile.config.key_file" required class="w-full" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'content'" :label="t('xnode.certificateContent')"><UTextarea v-model="profile.config.cert_content" :rows="5" class="w-full font-mono" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'content'" :label="t('xnode.keyContent')"><UTextarea v-model="profile.config.key_content" :rows="5" class="w-full font-mono" /></UFormField>
      </div>
      <UButton variant="ghost" color="neutral" icon="i-ph-trash-bold" :aria-label="t('common.delete')" @click="remove(id)" />
    </div>
    <UButton variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('xnode.addCertificate')" class="justify-self-start" @click="add" />
    <UFormField v-for="tag in inbounds ?? []" :key="tag" :label="t('nodeAdmin.certificateBinding', { tag })"><USelect :model-value="model.inboundCertificates?.[tag] ?? '_none'" :items="profiles" class="w-full" @update:model-value="bind(tag, $event)" /></UFormField>
  </div>
</template>

<style scoped>
.certificate-profile { display: flex; gap: 8px; align-items: start; }
.certificate-fields { flex: 1; display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 12px; }
@media(max-width:600px) { .certificate-fields { grid-template-columns: minmax(0,1fr); } }
</style>
