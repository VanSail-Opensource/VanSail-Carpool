<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'

const model = defineModel<XNodeSettings>({ required: true })
const { t } = useI18n()
const entries = computed(() => Object.entries(model.value.certificates ?? {}))
const modes = computed(() => ['http', 'dns', 'file', 'content', 'self'].map(value => ({ value, label: t(`xnode.certificateModes.${value}`) })))
function add(): void {
  const id = `certificate-${entries.value.length + 1}`
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
        <UFormField :label="t('xnode.domain')"><UInput v-model="profile.config.domain" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.email')"><UInput v-model="profile.config.email" type="email" class="w-full" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'dns'" :label="t('xnode.dnsProvider')"><UInput v-model="profile.provider" class="w-full" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'file'" :label="t('xnode.certificateFile')"><UInput v-model="profile.config.cert_file" class="w-full" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'file'" :label="t('xnode.keyFile')"><UInput v-model="profile.config.key_file" class="w-full" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'content'" :label="t('xnode.certificateContent')"><UTextarea v-model="profile.config.cert_content" :rows="5" class="w-full font-mono" /></UFormField>
        <UFormField v-if="profile.config.cert_mode === 'content'" :label="t('xnode.keyContent')"><UTextarea v-model="profile.config.key_content" :rows="5" class="w-full font-mono" /></UFormField>
      </div>
      <UButton variant="ghost" color="neutral" icon="i-ph-trash-bold" :aria-label="t('common.delete')" @click="remove(id)" />
    </div>
    <UButton variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('xnode.addCertificate')" class="justify-self-start" @click="add" />
  </div>
</template>

<style scoped>
.certificate-profile { display: flex; gap: 8px; align-items: start; }
.certificate-fields { flex: 1; display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 12px; }
@media(max-width:600px) { .certificate-fields { grid-template-columns: minmax(0,1fr); } }
</style>
