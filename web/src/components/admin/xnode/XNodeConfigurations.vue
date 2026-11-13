<script setup lang="ts">
import { computed, onScopeDispose, reactive, shallowRef, toRaw, useId, watch } from 'vue'
import { xnodeApi, type XNodeConfiguration, type XNodeSnippet } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { cloneForEditing, createUuid } from '@/utils/browserCompatibility'
import XNodeJsonEditor from '@/components/common/JsonDocumentField.vue'
import { shadowsocksConfiguration } from './configurationPresets'
import { saveConfigurationWithRecovery, saveSnippetWithRecovery } from './catalogCreation'

const props = defineProps<{ configurations: XNodeConfiguration[]; snippets: XNodeSnippet[]; busy: boolean; saveError?: string | null; perform: (action: () => Promise<unknown>) => Promise<boolean> }>()
const { t } = useI18n()
const formId = useId()
const mode = shallowRef<'configuration' | 'snippet'>('configuration')
const editing = shallowRef(false)
const removingId = shallowRef<string | null>(null)
const valid = shallowRef(true)
const validating = shallowRef(false)
const validation = shallowRef<{ valid: boolean; needsAgentValidation: boolean; detail?: string; errorCode?: string } | null>(null)
const draft = reactive({ id: '', name: '', revision: 0, kind: 'outbound' as 'outbound' | 'routing', snippetIds: [] as string[] })
const document = shallowRef<unknown>({})
const validationError = shallowRef<string | null>(null)
const validationNonce = shallowRef(0)
const submitting = shallowRef(false)
const blocked = computed(() => props.busy || submitting.value || removingId.value !== null)
let active = true
let creationId = ''
const recovery = new globalThis.AbortController()
let validationGeneration = 0
onScopeDispose(() => { active = false; validationGeneration++; recovery.abort() })
const snippetOptions = computed(() => props.snippets.map(item => ({ label: item.name, value: item.id })))
const kinds = computed(() => ['outbound', 'routing'].map(value => ({ value, label: t(`xnode.snippetKinds.${value}`) })))
function addShadowsocks() { if (blocked.value) return; editConfiguration(); document.value = shadowsocksConfiguration() }
function editConfiguration(item?: XNodeConfiguration): void {
  if (!active || blocked.value) return
  creationId = ''
  mode.value = 'configuration'; Object.assign(draft, { id: item?.id ?? '', name: item?.name ?? '', revision: item?.revision ?? 0, snippetIds: [...(item?.snippetIds ?? [])] })
  document.value = cloneForEditing(toRaw(item?.document ?? { inbounds: [], outbounds: [{ tag: 'direct', protocol: 'freedom' }, { UserOutbound: {} }], routing: { rules: [{ UserRouting: {} }] } }))
  editing.value = true; valid.value = true
}
function editSnippet(item?: XNodeSnippet): void {
  if (!active || blocked.value) return
  creationId = ''
  mode.value = 'snippet'; Object.assign(draft, { id: item?.id ?? '', name: item?.name ?? '', revision: item?.revision ?? 0, kind: item?.kind ?? 'outbound' })
  document.value = cloneForEditing(toRaw(item?.document ?? [])); editing.value = true; valid.value = true
}
watch([document, () => draft.snippetIds.slice(), editing, mode, valid, validationNonce], (_value, _old, cleanup) => {
  const generation = ++validationGeneration
  validation.value = null
  validationError.value = null
  validating.value = false
  if (!editing.value || mode.value !== 'configuration' || !valid.value) return
  const timer = globalThis.setTimeout(() => {
    validating.value = true
    void xnodeApi.validate(document.value as object, draft.snippetIds).then(value => { if (generation === validationGeneration) validation.value = value }).catch(() => { if (generation === validationGeneration) validationError.value = t('nodeAdmin.validationUnavailable') }).finally(() => { if (generation === validationGeneration) validating.value = false })
  }, 350)
  cleanup(() => { globalThis.clearTimeout(timer); validationGeneration++ })
})
async function save(): Promise<void> {
  if (!active || blocked.value || !valid.value || !draft.name || mode.value === 'configuration' && validation.value?.valid !== true) return
  const value = cloneForEditing(toRaw(document.value))
  const common = { id: draft.id, name: draft.name, revision: draft.revision }
  const configuration = { ...common, snippetIds: [...draft.snippetIds], document: value as Record<string, unknown> }
  const snippet = { ...common, kind: draft.kind, document: value as Record<string, unknown>[] }
  const selectedMode = mode.value
  const result = () => {
    const id = common.id || (creationId ||= createUuid())
    return selectedMode === 'configuration'
      ? saveConfigurationWithRecovery({ ...configuration, id }, recovery.signal)
      : saveSnippetWithRecovery({ ...snippet, id }, recovery.signal)
  }
  submitting.value = true
  try { if (await props.perform(result) && active) editing.value = false }
  finally { if (active) submitting.value = false }
}
async function removeConfiguration(item: XNodeConfiguration): Promise<void> {
  if (!active || blocked.value) return
  const id = item.id
  removingId.value = id
  try { await props.perform(() => xnodeApi.deleteConfiguration(id)) }
  finally { if (active) removingId.value = null }
}
</script>

<template>
  <div class="grid gap-5">
    <div class="flex flex-wrap justify-end gap-2"><UButton variant="outline" color="neutral" :disabled="blocked" :label="t('nodeAdmin.ss2022')" @click="addShadowsocks" /><UButton icon="i-ph-plus-bold" :disabled="blocked" :label="t('xnode.addConfiguration')" @click="editConfiguration()" /><UButton color="neutral" variant="outline" icon="i-ph-code-bold" :disabled="blocked" :label="t('xnode.addSnippet')" @click="editSnippet()" /></div>
    <p v-if="!configurations.length" class="text-muted">{{ t('xnode.noConfigurations') }}</p>
    <div v-for="item in configurations" :key="item.id" class="configuration-row"><strong>{{ item.name }}</strong><span class="text-muted text-xs">{{ t('xnode.revision', { revision: item.revision }) }}</span><div class="flex gap-1"><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :disabled="blocked" :aria-label="t('common.edit')" @click="editConfiguration(item)" /><UButton variant="ghost" color="error" icon="i-ph-trash-bold" :loading="removingId === item.id" :disabled="blocked" :aria-label="t('xnode.deleteConfiguration', { name: item.name })" @click="removeConfiguration(item)" /></div></div>
    <h3>{{ t('xnode.snippets') }}</h3>
    <p v-if="!snippets.length" class="text-muted">{{ t('xnode.noSnippets') }}</p>
    <div v-for="item in snippets" :key="item.id" class="configuration-row"><strong>{{ item.name }}</strong><span class="text-muted text-xs">{{ t(`xnode.snippetKinds.${item.kind}`) }}</span><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :disabled="blocked" :aria-label="t('common.edit')" @click="editSnippet(item)" /></div>
    <UModal :open="editing" :dismissible="!blocked" :close="!blocked" @update:open="!blocked && (editing = $event)" :title="t(mode === 'configuration' ? 'xnode.configuration' : 'xnode.snippet')" :ui="{ content: 'max-w-5xl' }">
      <template #body>
        <form :id="formId" class="grid gap-4" @submit.prevent="save">
          <UFormField :label="t('common.name')"><UInput v-model="draft.name" :disabled="blocked" required class="w-full" /></UFormField>
          <UFormField v-if="mode === 'snippet'" :label="t('xnode.snippetKind')"><USelect v-model="draft.kind" :disabled="blocked" :items="kinds" class="w-full" /></UFormField>
          <UFormField v-else :label="t('xnode.sharedSnippets')"><USelectMenu v-model="draft.snippetIds" :disabled="blocked" value-key="value" multiple :items="snippetOptions" class="w-full" /></UFormField>
          <XNodeJsonEditor v-model="document" :disabled="blocked" :label="t('xnode.document')" :rows="20" @valid="valid = $event" />
          <p v-if="mode === 'configuration'" class="text-xs text-muted">{{ t('xnode.userMarkersHint') }}</p>
          <UProgress v-if="validating" />
          <div v-if="validationError" class="grid gap-2"><p>{{ validationError }}</p><UButton variant="ghost" :label="t('common.tryAgain')" :disabled="blocked" @click="!blocked && validationNonce++" /></div>
          <pre v-if="validation?.detail" class="text-xs whitespace-pre-wrap break-words">{{ validation.detail }}</pre>
          <UAlert v-if="validation" :color="validation.valid ? 'success' : 'warning'" variant="soft" :title="t(validation.valid ? validation.needsAgentValidation ? 'xnode.nodeValidationRequired' : 'xnode.configurationValid' : 'xnode.configurationInvalid')" />
        </form>
      </template>
      <template #footer>
        <div class="grid w-full gap-3">
          <UAlert v-if="saveError" role="alert" color="warning" :title="saveError" />
          <div class="flex justify-end"><UButton type="submit" :form="formId" :loading="blocked" :disabled="blocked || !valid || validating || mode === 'configuration' && validation?.valid !== true" :label="t('common.save')" /></div>
        </div>
      </template>
    </UModal>
  </div>
</template>

<style scoped>
.configuration-row { display: grid; grid-template-columns: minmax(0,1fr) auto auto; gap: 12px; align-items: center; padding-block: 10px; border-bottom: 1px solid var(--line); }
.configuration-row strong { min-width: 0; overflow-wrap: anywhere; }
h3 { margin: 8px 0 0; font-size: 15px; }
</style>
