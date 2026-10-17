<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref, shallowRef } from 'vue'
import { xnodeApi } from '@/api/xnode'
import type { components } from '@/api/generated'
import { localizedError, useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

type Config = components['schemas']['XNodeRoutingTemplateConfig']
type Template = Config['templates'][number]
const { t } = useI18n()
const config = ref<Config | null>(null)
const loading = shallowRef(true), busy = shallowRef(false), loaded = shallowRef(false), saved = shallowRef(false)
const error = shallowRef<string | null>(null)
let reader: InstanceType<typeof globalThis.AbortController> | undefined, generation = 0, disposed = false
const valid = computed(() => loaded.value && !!config.value && config.value.templates.every(item => item.name.trim() && /^[A-Za-z0-9_-]{1,48}$/.test(item.id)))
async function load() {
  const ticket = ++generation; reader?.abort(); reader = new globalThis.AbortController(); loading.value = true; loaded.value = false; error.value = null
  try { const value = await xnodeApi.routingTemplates(reader.signal); if (!disposed && ticket === generation) { config.value = value; loaded.value = true } }
  catch (caught) { if (!disposed && ticket === generation) error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { if (!disposed && ticket === generation) loading.value = false }
}
async function save() {
  if (!valid.value || !config.value || busy.value) return
  const ticket = generation; busy.value = true; error.value = null; saved.value = false
  try { const value = await xnodeApi.saveRoutingTemplates(config.value); if (!disposed && ticket === generation) { config.value = value; saved.value = true } }
  catch (caught) { if (!disposed && ticket === generation) error.value = localizedError(caught, 'managedRouting.saveFailed') }
  finally { if (!disposed && ticket === generation) busy.value = false }
}
function add() { config.value?.templates.push({ id: createUuid(), name: '', match: {} }); saved.value = false }
function list(item: Template, key: 'domain' | 'ip' | 'protocol', value: string) { item.match[key] = value.split(/\r?\n/).map(s => s.trim()).filter(Boolean); saved.value = false }
function network(item: Template, key: 'tcp' | 'udp', enabled: boolean) { item.match.network = [...(item.match.network ?? []).filter(n => n !== key), ...(enabled ? [key] : [])]; saved.value = false }
onMounted(load)
onScopeDispose(() => { disposed = true; generation++; reader?.abort() })
</script>

<template>
  <div class="grid gap-5 min-w-0">
    <div class="flex items-center justify-between gap-3"><h3>{{ t('managedRouting.templatesTitle') }}</h3><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="loading" :disabled="busy" @click="load" /></div>
    <p class="text-sm text-muted">{{ t('managedRouting.templateDescription') }}</p>
    <UAlert v-if="error" color="warning" :title="error" />
    <UProgress v-if="loading" />
    <UButton v-else-if="!loaded" class="justify-self-start" variant="outline" :label="t('common.tryAgain')" @click="load" />
    <form v-if="loaded && config" class="grid gap-5" @submit.prevent="save">
      <fieldset v-for="(item, index) in config.templates" :key="item.id" :disabled="busy" class="grid gap-3 min-w-0 border-b border-default pb-5">
        <legend class="text-sm text-muted mb-2">{{ t('managedRouting.ruleNumber', { number: index + 1 }) }}</legend>
        <div class="grid gap-3 md:grid-cols-2"><UFormField :label="t('common.name')" required><UInput v-model="item.name" required :maxlength="128" class="w-full" /></UFormField><UFormField :label="t('managedRouting.ports')"><UInput v-model="item.match.port" class="w-full" :placeholder="t('managedRouting.portsHint')" /></UFormField></div>
        <div class="grid gap-3 md:grid-cols-2"><UFormField :label="t('managedRouting.domains')"><UTextarea :model-value="(item.match.domain ?? []).join('\n')" :rows="4" class="w-full font-mono" @update:model-value="list(item, 'domain', $event)" /></UFormField><UFormField :label="t('managedRouting.ips')"><UTextarea :model-value="(item.match.ip ?? []).join('\n')" :rows="4" class="w-full font-mono" @update:model-value="list(item, 'ip', $event)" /></UFormField></div>
        <div class="grid gap-3 md:grid-cols-2"><UFormField :label="t('managedRouting.protocols')"><UTextarea :model-value="(item.match.protocol ?? []).join('\n')" :rows="2" class="w-full" @update:model-value="list(item, 'protocol', $event)" /></UFormField><fieldset><legend class="text-sm font-medium">{{ t('managedRouting.networks') }}</legend><div class="flex gap-4 py-2"><UCheckbox v-for="value in (['tcp', 'udp'] as const)" :key="value" :model-value="item.match.network?.includes(value) ?? false" :label="value.toUpperCase()" @update:model-value="network(item, value, $event === true)" /></div></fieldset></div>
        <UButton type="button" class="justify-self-end" variant="ghost" color="neutral" icon="i-ph-trash-bold" :label="t('common.delete')" @click="config.templates.splice(index, 1)" />
      </fieldset>
      <p v-if="!config.templates.length" class="text-muted">{{ t('managedRouting.noTemplates') }}</p>
      <p v-if="saved" role="status" class="text-sm text-success">{{ t('memberNodes.saved') }}</p>
      <div class="flex justify-between gap-2"><UButton type="button" variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('managedRouting.addTemplate')" :disabled="busy || config.templates.length >= 64" @click="add" /><UButton type="submit" :label="t('common.save')" :loading="busy" :disabled="!valid" /></div>
    </form>
  </div>
</template>
