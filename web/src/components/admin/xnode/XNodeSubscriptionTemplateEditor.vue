<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeApi, type SubscriptionTemplate } from '@/api/xnode'
import JsonDocumentField from '@/components/common/JsonDocumentField.vue'
import { localizedError, useI18n } from '@/i18n'

const props = defineProps<{ item: SubscriptionTemplate; busy: boolean; error?: string | null; formatLocked?: boolean }>()
const emit = defineEmits<{ save: [item: SubscriptionTemplate]; close: [] }>()
const { t } = useI18n()
const name = shallowRef(props.item.name)
const format = shallowRef(props.item.format)
const document = shallowRef<unknown>(globalThis.structuredClone(props.item.document))
const valid = shallowRef(true)
const validating = shallowRef(false)
const preview = shallowRef<Awaited<ReturnType<typeof xnodeApi.previewSubscriptionTemplate>> | null>(null)
const previewError = shallowRef<string | null>(null)
const formats = computed(() => ['xray', 'sing-box', 'clash'].map(value => ({ value, label: t(`publicSubscription.formats.${value}`) })))
let generation = 0
const draft = computed<SubscriptionTemplate>(() => ({ id: props.item.id, name: name.value.trim(), format: format.value, document: document.value as Record<string, unknown> }))
const previewText = computed(() => {
  const raw = preview.value?.preview ?? ''
  if (format.value === 'clash') return raw
  try { return JSON.stringify(JSON.parse(raw), null, 2) } catch { return raw }
})
watch([document, format, valid], () => { generation++; preview.value = null; validating.value = false })
async function validate() {
  if (!valid.value || validating.value) return
  const ticket = ++generation
  validating.value = true; previewError.value = null
  try { const value = await xnodeApi.previewSubscriptionTemplate(draft.value); if (ticket === generation) preview.value = value }
  catch (caught) { if (ticket === generation) previewError.value = localizedError(caught, 'xnode.loadFailed') }
  finally { if (ticket === generation) validating.value = false }
}
function save() {
  if (!props.busy && valid.value && !validating.value && name.value.trim() && preview.value?.validation.valid) emit('save', draft.value)
}
onScopeDispose(() => { generation++ })
</script>

<template>
  <UModal :open="true" :title="t('nodeTemplates.editor')" :dismissible="!busy" :ui="{ content: 'max-w-5xl' }" @update:open="!$event && emit('close')">
    <template #body>
      <form class="grid gap-4" @submit.prevent="save">
        <UAlert v-if="props.error || previewError" color="warning" :title="props.error ?? previewError ?? ''" />
        <UFormField :label="t('common.name')"><UInput v-model="name" required maxlength="128" :disabled="busy" class="w-full" /></UFormField>
        <UFormField :label="t('publicSubscription.format')"><USelect v-model="format" :items="formats" :disabled="busy || formatLocked" class="w-full" /></UFormField>
        <p class="text-sm text-muted">{{ t('nodeTemplates.markerHint') }}</p>
        <JsonDocumentField v-model="document" :label="t('xnode.document')" :disabled="busy" :rows="14" @valid="valid = $event" />
        <div class="flex flex-wrap justify-end gap-2"><UButton variant="outline" color="neutral" :label="t('nodeTemplates.validate')" :loading="validating" :disabled="!valid || busy" @click="validate" /><UButton type="submit" :label="t('common.save')" :loading="busy" :disabled="!name.trim() || !valid || validating || !preview?.validation.valid" /></div>
        <UAlert v-if="preview" :color="preview.validation.valid ? 'success' : 'warning'" :title="t(preview.validation.valid ? 'xnode.configurationValid' : 'xnode.configurationInvalid')" />
        <p v-if="preview?.validation.needsAgentValidation" class="text-sm text-muted">{{ t('nodeTemplates.clientAssets') }}</p>
        <pre v-if="preview?.validation.detail" class="text-xs whitespace-pre-wrap break-words">{{ preview.validation.detail }}</pre>
        <details v-if="preview?.preview" open><summary>{{ t('nodeTemplates.preview') }}</summary><pre class="text-xs overflow-auto max-h-96 py-3">{{ previewText }}</pre></details>
      </form>
    </template>
  </UModal>
</template>
