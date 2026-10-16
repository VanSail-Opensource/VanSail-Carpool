<script setup lang="ts">
import { computed, onScopeDispose, reactive, shallowRef, watch } from 'vue'
import { xnodeApi, type XNodeConfiguration, type XNodeSnippet } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeJsonEditor from '@/components/common/JsonDocumentField.vue'
import { shadowsocksConfiguration } from './configurationPresets'

const props = defineProps<{ configurations: XNodeConfiguration[]; snippets: XNodeSnippet[]; busy: boolean; saveError?: string | null; perform: (action: () => Promise<unknown>) => Promise<boolean> }>()
const { t } = useI18n()
const mode = shallowRef<'configuration' | 'snippet'>('configuration')
const editing = shallowRef(false)
const valid = shallowRef(true)
const validating = shallowRef(false)
const validation = shallowRef<{ valid: boolean; needsAgentValidation: boolean; detail?: string; errorCode?: string } | null>(null)
const draft = reactive({ id: '', name: '', revision: 0, kind: 'outbound' as 'outbound' | 'routing', snippetIds: [] as string[] })
const document = shallowRef<unknown>({})
const validationError = shallowRef<string | null>(null)
const validationNonce = shallowRef(0)
let validationGeneration = 0
onScopeDispose(() => { validationGeneration++ })
const snippetOptions = computed(() => props.snippets.map(item => ({ label: item.name, value: item.id })))
const kinds = computed(() => ['outbound', 'routing'].map(value => ({ value, label: t(`xnode.snippetKinds.${value}`) })))
function addShadowsocks() { editConfiguration(); document.value = shadowsocksConfiguration() }
function editConfiguration(item?: XNodeConfiguration): void {
  mode.value = 'configuration'; Object.assign(draft, { id: item?.id ?? '', name: item?.name ?? '', revision: item?.revision ?? 0, snippetIds: [...(item?.snippetIds ?? [])] })
  document.value = item?.document ?? { inbounds: [], outbounds: [{ tag: 'direct', protocol: 'freedom' }, { UserOutbound: {} }], routing: { rules: [{ UserRouting: {} }] } }
  editing.value = true; valid.value = true
}
function editSnippet(item?: XNodeSnippet): void {
  mode.value = 'snippet'; Object.assign(draft, { id: item?.id ?? '', name: item?.name ?? '', revision: item?.revision ?? 0, kind: item?.kind ?? 'outbound' })
  document.value = item?.document ?? []; editing.value = true; valid.value = true
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
  if (!valid.value || !draft.name || mode.value === 'configuration' && validation.value?.valid !== true) return
  const result = mode.value === 'configuration'
    ? () => xnodeApi.saveConfiguration({ id: draft.id, name: draft.name, revision: draft.revision, snippetIds: draft.snippetIds, document: document.value as Record<string, unknown> })
    : () => xnodeApi.saveSnippet({ id: draft.id, name: draft.name, revision: draft.revision, kind: draft.kind, document: document.value as Record<string, unknown>[] })
  if (await props.perform(result)) editing.value = false
}
</script>

<template>
  <div class="grid gap-5">
    <div class="flex flex-wrap justify-end gap-2"><UButton variant="outline" color="neutral" :label="t('nodeAdmin.ss2022')" @click="addShadowsocks" /><UButton icon="i-ph-plus-bold" :label="t('xnode.addConfiguration')" @click="editConfiguration()" /><UButton color="neutral" variant="outline" icon="i-ph-code-bold" :label="t('xnode.addSnippet')" @click="editSnippet()" /></div>
    <p v-if="!configurations.length" class="text-muted">{{ t('xnode.noConfigurations') }}</p>
    <div v-for="item in configurations" :key="item.id" class="configuration-row"><strong>{{ item.name }}</strong><span class="text-muted text-xs">{{ t('xnode.revision', { revision: item.revision }) }}</span><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :aria-label="t('common.edit')" @click="editConfiguration(item)" /></div>
    <h3>{{ t('xnode.snippets') }}</h3>
    <p v-if="!snippets.length" class="text-muted">{{ t('xnode.noSnippets') }}</p>
    <div v-for="item in snippets" :key="item.id" class="configuration-row"><strong>{{ item.name }}</strong><span class="text-muted text-xs">{{ t(`xnode.snippetKinds.${item.kind}`) }}</span><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :aria-label="t('common.edit')" @click="editSnippet(item)" /></div>
    <UModal v-model:open="editing" :title="t(mode === 'configuration' ? 'xnode.configuration' : 'xnode.snippet')" :ui="{ content: 'max-w-5xl' }">
      <template #body><form class="grid gap-4" @submit.prevent="save"><UAlert v-if="saveError" color="warning" :title="saveError" /><UFormField :label="t('common.name')"><UInput v-model="draft.name" required class="w-full" /></UFormField><UFormField v-if="mode === 'snippet'" :label="t('xnode.snippetKind')"><USelect v-model="draft.kind" :items="kinds" class="w-full" /></UFormField><UFormField v-else :label="t('xnode.sharedSnippets')"><USelectMenu v-model="draft.snippetIds" value-key="value" multiple :items="snippetOptions" class="w-full" /></UFormField><XNodeJsonEditor v-model="document" :label="t('xnode.document')" :rows="20" @valid="valid = $event" /><p v-if="mode === 'configuration'" class="text-xs text-muted">{{ t('xnode.userMarkersHint') }}</p><UProgress v-if="validating" /><div v-if="validationError" class="grid gap-2"><p>{{ validationError }}</p><UButton variant="ghost" :label="t('common.tryAgain')" @click="validationNonce++" /></div><pre v-if="validation?.detail" class="text-xs whitespace-pre-wrap break-words">{{ validation.detail }}</pre><UAlert v-if="validation" :color="validation.valid ? 'success' : 'warning'" variant="soft" :title="t(validation.valid ? validation.needsAgentValidation ? 'xnode.nodeValidationRequired' : 'xnode.configurationValid' : 'xnode.configurationInvalid')" /><div class="flex justify-end"><UButton type="submit" :loading="busy" :disabled="!valid || validating || mode === 'configuration' && validation?.valid !== true" :label="t('common.save')" /></div></form></template>
    </UModal>
  </div>
</template>

<style scoped>
.configuration-row { display: grid; grid-template-columns: minmax(0,1fr) auto auto; gap: 12px; align-items: center; padding-block: 10px; border-bottom: 1px solid var(--line); }
h3 { margin: 8px 0 0; font-size: 15px; }
</style>
