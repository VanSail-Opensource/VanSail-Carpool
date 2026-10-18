<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeApi, type SubscriptionTemplateConfig } from '@/api/xnode'
import { localizedError, useI18n } from '@/i18n'
import { subscriptionFormats } from './subscriptionTemplatePresets'
const config = defineModel<SubscriptionTemplateConfig>({ required: true })
defineProps<{ busy: boolean }>()
const { t } = useI18n()
const rules = computed(() => config.value.userAgentRules ?? [])
const templates = computed(() => config.value.templates.map(item => ({ value: item.id, label: `${item.name} · ${t(`publicSubscription.formats.${item.format}`)}` })))
const formats = computed(() => [{ value: '_auto', label: t('nodeTemplates.automatic') }, ...subscriptionFormats.map(value => ({ value, label: t(`publicSubscription.formats.${value}`) }))])
const userAgent = shallowRef('')
const format = shallowRef('_auto')
const result = shallowRef<Awaited<ReturnType<typeof xnodeApi.previewSubscriptionSelection>> | null>(null)
const error = shallowRef<string | null>(null)
const pending = shallowRef(false)
let generation = 0
let controller: InstanceType<typeof globalThis.AbortController> | undefined
function invalidate(): void { generation++; controller?.abort(); result.value = null; pending.value = false; error.value = null }
watch([config, userAgent, format], invalidate, { deep: true })
function move(index: number, delta: number): void {
  const next = [...rules.value]; const [item] = next.splice(index, 1)
  if (item) { next.splice(index + delta, 0, item); config.value.userAgentRules = next }
}
function add(): void { if (templates.value[0] && rules.value.length < 64) config.value.userAgentRules = [...rules.value, { pattern: '', templateId: templates.value[0].value }] }
function remove(index: number): void { config.value.userAgentRules = rules.value.filter((_, i) => i !== index) }
async function preview(): Promise<void> {
  invalidate(); controller = new globalThis.AbortController(); const ticket = generation
  pending.value = true
  try { const value = await xnodeApi.previewSubscriptionSelection({ config: config.value, userAgent: userAgent.value, format: format.value === '_auto' ? '' : format.value }, controller.signal); if (ticket === generation) result.value = value }
  catch (caught) { if (ticket === generation) error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { if (ticket === generation) pending.value = false }
}
onScopeDispose(invalidate)
</script>
<template>
  <section class="grid gap-3 min-w-0">
    <div class="flex justify-between items-center gap-3"><h4>{{ t('nodeTemplates.rules') }}</h4><UButton type="button" variant="outline" color="neutral" icon="i-ph-plus" :label="t('nodeTemplates.addRule')" :disabled="busy || !templates.length || rules.length >= 64" @click="add" /></div>
    <p class="text-sm text-muted">{{ t('nodeTemplates.rulesHint') }}</p>
    <p v-if="!rules.length" class="text-sm text-muted">{{ t('nodeTemplates.noRules') }}</p>
    <div v-for="(rule, index) in rules" :key="index" class="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
      <UFormField :label="t('nodeTemplates.pattern')"><UInput v-model="rule.pattern" :disabled="busy" :maxlength="2048" class="w-full font-mono" /></UFormField>
      <UFormField :label="t('nodeTemplates.template')"><USelect v-model="rule.templateId" :disabled="busy" :items="templates" class="w-full" /></UFormField>
      <div class="flex gap-1 items-end"><UButton type="button" icon="i-ph-arrow-up" :aria-label="t('nodeTemplates.moveUp')" :disabled="busy || index === 0" color="neutral" variant="ghost" @click="move(index, -1)" /><UButton type="button" icon="i-ph-arrow-down" :aria-label="t('nodeTemplates.moveDown')" :disabled="busy || index === rules.length - 1" color="neutral" variant="ghost" @click="move(index, 1)" /><UButton type="button" icon="i-ph-trash" :aria-label="t('common.delete')" :disabled="busy" color="neutral" variant="ghost" @click="remove(index)" /></div>
    </div>
    <div class="grid gap-3 sm:grid-cols-2"><UFormField :label="t('nodeTemplates.userAgent')"><UInput v-model="userAgent" :disabled="busy" :maxlength="8192" class="w-full" /></UFormField><UFormField :label="t('publicSubscription.format')"><USelect v-model="format" :items="formats" :disabled="busy" class="w-full" /></UFormField></div>
    <UButton class="justify-self-start" type="button" :label="t('nodeTemplates.testSelection')" :loading="pending" :disabled="busy" @click="preview" />
    <UAlert v-if="error || result && !result.validation.valid" color="warning" :title="error ?? t('xnode.configurationInvalid')" :description="result?.validation.detail" />
    <p v-if="result?.validation.valid" role="status" class="text-sm">{{ t('nodeTemplates.selection', { format: t(`publicSubscription.formats.${result.selection.format}`), template: config.templates.find(item => item.id === result?.selection.templateId)?.name ?? t('nodeTemplates.useDefault'), source: t(`nodeTemplates.sources.${result.selection.source}`) }) }}</p>
  </section>
</template>
