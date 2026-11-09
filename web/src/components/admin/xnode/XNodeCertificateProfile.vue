<script setup lang="ts">
import { computed, onScopeDispose, shallowRef } from 'vue'
import { localizedError, useI18n } from '@/i18n'
import { certificateErrors, certificateMode, certificateModes, maxCertificatePEMBytes, type CertificateProfile, type CertificateProviders } from './certificateForm'

const model = defineModel<CertificateProfile>({ required: true })
const props = withDefaults(defineProps<{ id: string; providers: CertificateProviders; disabled?: boolean; providersLoading?: boolean }>(), { disabled: false, providersLoading: false })
const { t } = useI18n()
const errors = computed(() => certificateErrors(model.value, props.providers))
const mode = computed(() => certificateMode(model.value.config))
const modes = computed(() => certificateModes.map(value => ({ value, label: t(`xnode.certificateModes.${value}`) })))
const providers = computed(() => {
  const values = Object.entries(props.providers).map(([value, provider]) => ({ value, label: `${value} (${provider.type})` }))
  if (model.value.config.dns_provider) values.push({ value: '_legacy', label: t('certificates.legacyProvider', { name: model.value.config.dns_provider }) })
  if (model.value.provider && !props.providers[model.value.provider]) values.push({ value: model.value.provider, label: model.value.provider })
  return values
})
const importError = shallowRef<string | null>(null)
const versions = { cert_content: 0, key_content: 0 }
onScopeDispose(() => { versions.cert_content++; versions.key_content++ })
function update(key: keyof CertificateProfile['config'], value: unknown): void {
  if (key === 'cert_content' || key === 'key_content') versions[key]++
  if (key === 'cert_mode') { versions.cert_content++; versions.key_content++ }
  const config = { ...model.value.config, [key]: value }
  if (key !== 'cert_mode' && !model.value.config.cert_mode?.trim() && certificateModes.some(item => item === mode.value)) config.cert_mode = mode.value
  model.value = { ...model.value, config }
}
function error(key: keyof typeof errors.value): string | undefined { return errors.value[key] ? t(`certificates.${errors.value[key]}`) : undefined }
async function importPEM(key: 'cert_content' | 'key_content', event: globalThis.Event): Promise<void> {
  const input = event.target as globalThis.HTMLInputElement, file = input.files?.[0], ticket = ++versions[key]
  if (!file) return
  importError.value = null
  try {
    if (file.size > maxCertificatePEMBytes) { importError.value = t('certificates.fileTooLarge'); return }
    const value = await file.text()
    if (ticket === versions[key] && !props.disabled) update(key, value)
  } catch (caught) { if (ticket === versions[key]) importError.value = localizedError(caught, 'certificates.importFailed') }
  finally { input.value = '' }
}
</script>

<template>
  <div class="grid min-w-0 gap-3 sm:grid-cols-2" data-certificate-profile-fields>
    <UFormField :label="t('xnode.profileId')"><UInput :model-value="id" readonly class="w-full" /></UFormField>
    <UFormField :label="t('xnode.certificateMode')" :error="error('cert_mode')"><USelect :model-value="mode" :items="modes" :disabled="disabled" class="w-full" @update:model-value="update('cert_mode', $event)" /></UFormField>
    <UFormField v-if="['http', 'dns', 'content', 'self'].includes(mode)" :label="t('xnode.domain')" :description="mode === 'content' ? t('certificates.contentDomainHint') : undefined" :error="error('domain')" :required="mode !== 'content'"><UInput :model-value="model.config.domain" :disabled="disabled" class="w-full" @update:model-value="update('domain', $event)" /></UFormField>
    <UFormField v-if="['http', 'dns'].includes(mode)" :label="t('xnode.email')" :description="t('certificates.emailHint')" :error="error('email')"><UInput :model-value="model.config.email" type="email" :disabled="disabled" class="w-full" @update:model-value="update('email', $event)" /></UFormField>
    <UFormField v-if="['http', 'dns'].includes(mode)"><USwitch :model-value="!!model.config.auto_tls" :label="t('nodeAdmin.autoTls')" :disabled="disabled" @update:model-value="update('auto_tls', $event)" /></UFormField>
    <template v-if="mode === 'dns'">
      <UFormField :label="t('xnode.dnsProvider')" :error="error('provider')" required><USelect :model-value="model.provider || (model.config.dns_provider ? '_legacy' : undefined)" :items="providers" :loading="providersLoading" :disabled="disabled || providersLoading" :placeholder="t('certificates.chooseProvider')" class="w-full" @update:model-value="model = { ...model, provider: $event === '_legacy' ? '' : String($event) }" /></UFormField>
      <UButton to="/admin/xnode?xnodeTab=global" target="_blank" rel="noopener noreferrer" type="button" color="neutral" variant="link" trailing-icon="i-ph-arrow-square-out" class="justify-self-start sm:col-span-2" :label="t('certificates.manageProviders')" />
    </template>
    <template v-if="mode === 'file'">
      <UFormField :label="t('xnode.certificateFile')" :error="error('cert_file')" required><UInput :model-value="model.config.cert_file" :disabled="disabled" class="w-full" @update:model-value="update('cert_file', $event)" /></UFormField>
      <UFormField :label="t('xnode.keyFile')" :error="error('key_file')" required><UInput :model-value="model.config.key_file" :disabled="disabled" class="w-full" @update:model-value="update('key_file', $event)" /></UFormField>
    </template>
    <template v-if="mode === 'content'">
      <p class="text-xs text-muted sm:col-span-2">{{ t('certificates.pemHint') }}</p>
      <UFormField v-for="key in (['cert_content', 'key_content'] as const)" :key="key" :label="t(key === 'cert_content' ? 'xnode.certificateContent' : 'xnode.keyContent')" :error="error(key)" required class="min-w-0">
        <UTextarea :model-value="model.config[key]" :rows="5" :disabled="disabled" spellcheck="false" autocomplete="off" class="w-full font-mono" @update:model-value="update(key, $event)" />
        <label class="mt-2 grid gap-1 text-xs text-muted"><span>{{ t(key === 'cert_content' ? 'certificates.importCertificate' : 'certificates.importKey') }}</span><UInput type="file" accept=".pem,.crt,.cer,.key,text/plain" :disabled="disabled" class="w-full" @change="importPEM(key, $event)" /></label>
      </UFormField>
      <p v-if="importError" role="alert" class="text-sm text-error sm:col-span-2">{{ importError }}</p>
    </template>
  </div>
</template>
