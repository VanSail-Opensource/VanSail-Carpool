<script setup lang="ts">
import { computed, onScopeDispose, reactive, shallowRef, watch } from 'vue'
import { xnodeApi, type XNodeEndpoint, type XNodeEndpointField, type XNode, type XNodeConfiguration } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { nodeInboundItems } from './nodeFormOptions'
import { endpointDraft, endpointOptions, endpointOptionErrors, endpointPayload } from './endpointForm'
import { useEndpointPreview } from './useEndpointPreview'
import { endpointError } from './endpointLabels'
import XNodeEndpointOptions from './XNodeEndpointOptions.vue'
import XNodeEndpointCompatibility from './XNodeEndpointCompatibility.vue'
const props = defineProps<{ item?: XNodeEndpoint; nodes: XNode[]; configurations: XNodeConfiguration[]; busy: boolean; saveError?: string | null; perform: (action: () => Promise<unknown>) => Promise<boolean> }>()
const emit = defineEmits<{ saved: [] }>()
const { t } = useI18n()
const submitting = shallowRef(false)
const blocked = computed(() => props.busy || submitting.value)
let active = true
onScopeDispose(() => { active = false })
const draft = reactive(endpointDraft(props.item, props.nodes[0]?.id))
const nodes = computed(() => props.nodes.map(item => ({ value: item.id, label: item.name })))
const options = computed({ get: () => endpointOptions(draft), set: value => { draft.overrides = { ...draft.overrides, clientOptions: value } } })
const fields = shallowRef<XNodeEndpointField[]>([])
const localErrors = computed(() => endpointOptionErrors(fields.value, options.value))
const preview = useEndpointPreview(() => draft, () => Object.keys(localErrors.value).length > 0, () => fields.value)
const inbounds = computed(() => {
  if (preview.result.value) return preview.result.value.inbounds.map(item => ({ value: item.tag, label: item.tag }))
  const items = nodeInboundItems(draft.nodeId, props.nodes, props.configurations)
  return draft.inboundTag && !items.some(item => item.value === draft.inboundTag) ? [{ value: draft.inboundTag, label: draft.inboundTag }, ...items] : items
})
watch(() => preview.result.value?.fields, value => { fields.value = value ?? [] }, { flush: 'sync' })
const fieldErrors = computed(() => Object.fromEntries((preview.result.value?.fieldErrors ?? []).map(item => [item.field, item.code])))
const optionErrors = computed(() => ({ ...Object.fromEntries(Object.entries(fieldErrors.value).filter(([key]) => key.startsWith('clientOptions.')).map(([key, code]) => [key.slice(14), code])), ...localErrors.value }))
const metadata = computed(() => preview.result.value?.metadata)
const error = (field: string) => fieldErrors.value[field] ? endpointError(fieldErrors.value[field]) : undefined
watch(() => draft.nodeId, () => { draft.inboundTag = '' })
watch(() => preview.result.value?.inbounds, items => { if (items && draft.inboundTag && !items.some(item => item.tag === draft.inboundTag)) draft.inboundTag = '' })
async function save() {
  if (!active || blocked.value || !preview.valid.value) return
  const payload = endpointPayload(draft, fields.value)
  submitting.value = true
  try {
    const saved = await props.perform(() => xnodeApi.saveEndpoint(payload))
    if (!active) return
    if (saved) emit('saved')
    else await preview.load()
  } finally { if (active) submitting.value = false }
}
</script>
<template>
  <form class="grid min-w-0 gap-5" @submit.prevent="save">
    <UAlert v-if="saveError" color="warning" :title="t('endpointOptions.saveFailed')" :description="saveError !== t('xnode.saveFailed') ? saveError : undefined" />
    <div class="grid gap-3 sm:grid-cols-2">
      <UFormField :label="t('common.name')" :error="error('name')"><UInput v-model="draft.name" required :disabled="blocked" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.node')" :error="error('nodeId')"><USelect v-model="draft.nodeId" :items="nodes" :disabled="blocked" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.inboundTag')" :error="error('inboundTag')"><USelect v-model="draft.inboundTag" :items="inbounds" :disabled="blocked || !inbounds.length" :placeholder="t('endpointOptions.chooseInbound')" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.address')" :error="error('address')"><UInput v-model="draft.address" required :disabled="blocked" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.port')" :error="error('port')"><UInput v-model.number="draft.port" type="number" min="1" max="65535" step="1" :disabled="blocked" class="w-full" /></UFormField>
      <div class="flex flex-wrap items-center gap-4"><USwitch v-model="draft.enabled" :disabled="blocked" :label="t('common.enabled')" /><USwitch v-model="draft.hidden" :disabled="blocked" :label="t('xnode.hiddenEndpoint')" /></div>
    </div>
    <p v-if="metadata?.protocol" class="text-sm text-muted">{{ t('endpointOptions.metadata', { protocol: metadata.protocol, transport: metadata.transport || t('endpointOptions.default'), security: metadata.security || t('endpointOptions.none') }) }}</p>
    <div :aria-busy="preview.pending.value" class="grid min-w-0 gap-4">
      <p v-if="preview.pending.value" role="status" class="text-sm text-muted">{{ t('endpointOptions.checking') }}</p>
      <UAlert v-if="preview.error.value" color="warning" :title="preview.error.value"><template #actions><UButton color="neutral" variant="outline" :label="t('common.tryAgain')" @click="preview.load" /></template></UAlert>
      <UAlert v-if="error('overrides')" color="warning" :title="error('overrides')" />
      <UAlert v-if="error('clientOptions')" color="warning" :title="error('clientOptions')" />
      <XNodeEndpointOptions v-if="preview.result.value" v-model="options" :fields="preview.result.value.fields" :errors="optionErrors" :disabled="blocked" />
      <XNodeEndpointCompatibility v-if="preview.result.value && !preview.pending.value && !preview.error.value" :clients="preview.result.value.clients" />
    </div>
    <UButton type="submit" :loading="blocked" :disabled="!preview.valid.value || blocked" :label="t('common.save')" class="justify-self-end" />
  </form>
</template>
