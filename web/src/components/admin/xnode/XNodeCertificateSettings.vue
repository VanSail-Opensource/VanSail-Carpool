<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import type { XNodeSettings, XNodeMetrics } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { certificateBindingsValid, certificateBoundInbounds, certificateErrors, type CertificateProfile, type CertificateProviders } from './certificateForm'
import { readCertificateStatus } from './certificateStatus'
import XNodeCertificateProfile from './XNodeCertificateProfile.vue'
import XNodeCertificateStatus from './XNodeCertificateStatus.vue'
import XNodeCertificateLifecycle from './XNodeCertificateLifecycle.vue'

const model = defineModel<XNodeSettings>({ required: true })
const props = withDefaults(defineProps<{ inbounds?: string[]; providers: CertificateProviders; providersLoading?: boolean; nodeId?: string; revision?: number; metric?: XNodeMetrics['nodes'][number]; saved?: boolean; savedProfileIds?: string[] }>(), { inbounds: () => [], providersLoading: false, nodeId: '', revision: 0, saved: false, savedProfileIds: () => [], metric: undefined })
const emit = defineEmits<{ valid: [value: boolean] }>()
const { t } = useI18n()
const entries = computed(() => Object.entries(model.value.certificates ?? {}))
const profiles = computed(() => [{ value: '_none', label: t('nodeAdmin.noCertificate') }, ...entries.value.map(([id]) => ({ value: id, label: id }))])
const bindings = computed(() => [...new Set([...props.inbounds, ...Object.keys(model.value.inboundCertificates ?? {})])])
const busy = reactive<Record<string, boolean>>({})
const fresh = computed(() => !!props.metric?.connected && !props.metric.stale)
const capable = computed(() => props.metric?.capabilities?.includes('certificate_lifecycle_v1') === true)
const reason = computed(() => !props.saved || !props.nodeId || !Number.isSafeInteger(props.revision) || props.revision < 1 ? t('certificates.saveFirst') : !capable.value ? t('certificates.upgradeRequired')
  : !fresh.value ? t('certificates.nodeUnavailable') : '')
const valid = computed(() => entries.value.length <= 64 && entries.value.every(([, profile]) => !Object.keys(certificateErrors(profile, props.providers)).length && !(profile.config.cert_mode === 'dns' && !!profile.provider && props.providersLoading))
  && certificateBindingsValid(model.value, props.inbounds) && !Object.values(busy).some(Boolean))
watch(valid, value => emit('valid', value), { immediate: true, flush: 'sync' })
function update(id: string, profile: CertificateProfile): void { model.value = { ...model.value, certificates: { ...model.value.certificates, [id]: profile } } }
function bound(id: string): string[] { return certificateBoundInbounds(model.value, id) }
function canRun(profile: CertificateProfile): boolean { return !reason.value && !Object.keys(certificateErrors(profile, props.providers)).length }
function status(id: string): unknown {
  const value = props.metric?.certificates[id]
  const progress = props.metric?.certificateProgress
  const current = value && typeof value === 'object' ? value as Record<string, unknown> : {}
  const terminal = progress && current.command_id === progress.command_id && ['idle', 'failed'].includes(String(current.phase))
  if (progress?.profile_id === id && !terminal && Date.now() - Date.parse(progress.received_at) < 60_000) return { ...current, phase: progress.phase }
  return value
}
function bind(tag: string, id: string) {
  const bindings = { ...model.value.inboundCertificates }
  if (id === '_none') delete bindings[tag]
  else bindings[tag] = id
  model.value = { ...model.value, inboundCertificates: bindings }
}
function add(): void {
  if (entries.value.length >= 64) return
  let suffix = entries.value.length + 1
  while (model.value.certificates?.[`certificate-${suffix}`]) suffix++
  const id = `certificate-${suffix}`
  model.value = { ...model.value, certificates: { ...model.value.certificates, [id]: { provider: '', config: { domain: '', cert_mode: 'http', auto_tls: true } } } }
}
function remove(id: string): void {
  if (bound(id).length || busy[id]) return
  const next = { ...model.value.certificates }; delete next[id]
  delete busy[id]
  model.value = { ...model.value, certificates: next }
}
</script>

<template>
  <div class="grid min-w-0 gap-4 py-4 [overflow-wrap:anywhere]">
    <p v-if="!entries.length" class="text-sm text-muted">{{ t('certificates.empty') }}</p>
    <div v-for="[id, profile] in entries" :key="id" class="certificate-profile" :data-certificate-profile="id">
      <div class="flex items-center justify-between gap-3"><strong class="text-sm break-all">{{ id }}</strong><UButton type="button" variant="ghost" color="neutral" icon="i-ph-trash-bold" :disabled="!!bound(id).length || busy[id]" :aria-label="t('certificates.removeProfile', { name: id })" @click="remove(id)" /></div>
      <p v-if="bound(id).length" class="text-xs text-muted">{{ t('certificates.boundTo', { tags: bound(id).join(', ') }) }}</p>
      <XNodeCertificateProfile :model-value="profile" :id="id" :providers="providers" :providers-loading="providersLoading" :disabled="busy[id]" @update:model-value="update(id, $event)" />
      <XNodeCertificateStatus :value="status(id)" :fresh="fresh" />
      <XNodeCertificateLifecycle v-if="nodeId && savedProfileIds.includes(id) && ['http', 'dns'].includes(profile.config.cert_mode)" :node-id="nodeId" :profile-id="id" :revision="revision" :can-run="canRun(profile)" :active="readCertificateStatus(status(id))?.active ?? false" :reason="reason" @busy="busy[id] = $event" />
      <p v-else class="text-xs text-muted">{{ t('certificates.saveToApply') }}</p>
    </div>
    <UButton type="button" variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('xnode.addCertificate')" :disabled="entries.length >= 64 || Object.values(busy).some(Boolean)" class="justify-self-start" @click="add" />
    <p class="text-xs text-muted">{{ t('certificates.bindingHint') }}</p>
    <UFormField v-for="tag in bindings" :key="tag" :label="t('nodeAdmin.certificateBinding', { tag })" :error="!inbounds.includes(tag) || model.inboundCertificates?.[tag] && !model.certificates?.[model.inboundCertificates[tag]!] ? t('certificates.bindingInvalid') : undefined"><USelect :model-value="model.inboundCertificates?.[tag] ?? '_none'" :items="profiles" :disabled="Object.values(busy).some(Boolean)" class="w-full" @update:model-value="bind(tag, $event)" /></UFormField>
  </div>
</template>

<style scoped>
.certificate-profile { display: grid; min-width: 0; gap: 1rem; border-bottom: 1px solid var(--line); padding-bottom: 1.25rem; }
</style>
