<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref, shallowRef, toRaw } from 'vue'
import { xnodeApi, type SubscriptionTemplate, type SubscriptionTemplateConfig } from '@/api/xnode'
import { localizedError, useI18n } from '@/i18n'
import XNodeSubscriptionTemplateEditor from './XNodeSubscriptionTemplateEditor.vue'
import XNodeSubscriptionRules from './XNodeSubscriptionRules.vue'
import { templateFormats, subscriptionFormats, subscriptionPreset } from './subscriptionTemplatePresets'

const { t } = useI18n()
const config = ref<SubscriptionTemplateConfig | null>(null)
const editing = shallowRef<SubscriptionTemplate | null>(null)
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const saved = shallowRef(false)
let active = true
const formats = templateFormats
const enabledFormats = subscriptionFormats
const loaded = shallowRef(false)
let controller: InstanceType<typeof globalThis.AbortController> | undefined
let generation = 0
const options = (format: string) => [{ value: '_builtin', label: t('nodeTemplates.builtin') }, ...(config.value?.templates.filter(item => item.format === format).map(item => ({ value: item.id, label: item.name })) ?? [])]
const assigned = computed(() => new Set([...Object.values(config.value?.defaults ?? {}), ...Object.values(config.value?.assignments ?? {}).flatMap(value => Object.values(value)), ...(config.value?.userAgentRules ?? []).map(rule => rule.templateId)]))
async function load() {
  controller?.abort(); controller = new globalThis.AbortController(); const ticket = ++generation
  loaded.value = false; loading.value = true; error.value = null
  try { const value = await xnodeApi.subscriptionTemplates(controller.signal); if (active && ticket === generation) { config.value = value; loaded.value = true } }
  catch (caught) { if (active && ticket === generation) error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { if (active && ticket === generation) loading.value = false }
}
async function save(next = config.value) {
  if (!next || busy.value || !loaded.value || loading.value) return false
  busy.value = true; error.value = null; saved.value = false
  try { const value = await xnodeApi.saveSubscriptionTemplates(next); if (active) { config.value = value; saved.value = true }; return true }
  catch (caught) { if (active) error.value = localizedError(caught, 'nodeTemplates.saveFailed'); return false }
  finally { if (active) busy.value = false }
}
function add() {
  editing.value = { id: globalThis.crypto.randomUUID(), name: '', format: 'xray', document: subscriptionPreset('xray') }
}
function edit(item: SubscriptionTemplate) { editing.value = globalThis.structuredClone(toRaw(item)) }
async function saveTemplate(item: SubscriptionTemplate) {
  if (!config.value) return
  const next = { ...config.value, templates: [...config.value.templates.filter(value => value.id !== item.id), item] }
  if (await save(next)) editing.value = null
}
async function remove(id: string) { if (config.value && !assigned.value.has(id)) await save({ ...config.value, templates: config.value.templates.filter(item => item.id !== id) }) }
function setDefault(format: string, id: string) { if (config.value) config.value.defaults = { ...config.value.defaults, [format]: id === '_builtin' ? '' : id } }
onMounted(load)
onScopeDispose(() => { active = false; generation++; controller?.abort() })
</script>

<template>
  <div class="grid gap-5 min-w-0">
    <div class="flex justify-between gap-3"><h3>{{ t('nodeTemplates.title') }}</h3><div class="flex gap-2"><UButton icon="i-ph-arrow-clockwise" variant="ghost" color="neutral" :aria-label="t('common.refresh')" :disabled="busy" :loading="loading" @click="load" /><UButton icon="i-ph-plus-bold" :label="t('nodeTemplates.add')" :disabled="!loaded || busy || loading" @click="add" /></div></div>
    <UAlert v-if="error" color="warning" :title="error" />
    <p v-if="saved" role="status" class="text-sm text-success">{{ t('nodeTemplates.saved') }}</p>
    <UProgress v-if="loading" />
    <template v-if="config && loaded && !loading">
      <p class="text-sm text-muted">{{ t('nodeTemplates.description') }}</p>
      <p v-if="!config.templates.length" class="text-muted">{{ t('nodeTemplates.empty') }}</p>
      <div v-for="item in config.templates" :key="item.id" class="flex justify-between items-center gap-3 py-2 border-b border-default"><div class="min-w-0"><strong>{{ item.name }}</strong><p class="text-xs text-muted">{{ t(item.format === 'empty-http' ? 'nodeTemplates.emptyHTTP' : `publicSubscription.formats.${item.format}`) }}</p></div><div class="flex gap-1"><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :aria-label="t('common.edit')" :disabled="busy" @click="edit(item)" /><UTooltip :text="t('nodeTemplates.removeHint')"><UButton variant="ghost" color="neutral" icon="i-ph-trash-bold" :aria-label="t('common.delete')" :disabled="busy || assigned.has(item.id)" @click="remove(item.id)" /></UTooltip></div></div>
      <form class="grid gap-4" @submit.prevent="save()">
        <div class="flex flex-wrap gap-4"><USwitch v-for="format in enabledFormats" :key="format" v-model="config.formats[format]" :label="t(`publicSubscription.formats.${format}`)" :disabled="busy" /></div>
        <div class="grid gap-3 sm:grid-cols-3"><UFormField v-for="format in formats" :key="format" :label="t(`publicSubscription.formats.${format}`)"><USelect :model-value="config.defaults[format] || '_builtin'" :items="options(format)" :disabled="busy" class="w-full" @update:model-value="setDefault(format, $event)" /></UFormField></div>
        <XNodeSubscriptionRules v-model="config" :busy="busy" />
        <UButton type="submit" class="justify-self-end" :disabled="!loaded || loading" :loading="busy" :label="t('common.save')" />
      </form>
    </template>
    <XNodeSubscriptionTemplateEditor v-if="editing" :key="editing.id" :item="editing" :format-locked="assigned.has(editing.id)" :busy="busy" :error="error" @save="saveTemplate" @close="editing = null" />
  </div>
</template>
