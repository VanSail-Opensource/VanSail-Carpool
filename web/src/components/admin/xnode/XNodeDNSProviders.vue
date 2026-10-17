<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeGlobal } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { providerCatalog } from './providerCatalog'

const model = defineModel<XNodeGlobal['providers']>({ required: true })
const { t } = useI18n()
const entries = computed(() => Object.entries(model.value ?? {}).map(([id, value]) => ({ id, ...value })))
const types = computed(() => [...new Set([...providerCatalog.map(item => item.id), ...entries.value.map(item => item.type)])].map(value => ({ value, label: value })))
function fields(item: typeof entries.value[number]) {
  const spec = providerCatalog.find(value => value.id === item.type || value.aliases.includes(item.type))
  return [...new Set([...(spec?.fields ?? []), ...Object.keys(item.credentials ?? {})])]
}
function add() {
  let id = 'provider-1'
  for (let index = 2; model.value?.[id]; index++) id = `provider-${index}`
  model.value = { ...model.value, [id]: { type: 'cloudflare', credentials: {} } }
}
function remove(id: string) { const next = { ...model.value }; delete next[id]; model.value = next }
function updateType(id: string, type: string) { model.value = { ...model.value, [id]: { type, credentials: {} } } }
function updateSecret(id: string, key: string, value: string) {
  const previous = model.value[id]!
  model.value = { ...model.value, [id]: { ...previous, credentials: { ...previous.credentials, [key]: value } } }
}
</script>

<template>
  <section class="grid gap-4 py-3">
    <p class="text-sm text-muted">{{ t('xnode.providerSecretsHint') }}</p>
    <div v-for="item in entries" :key="item.id" class="grid gap-3 border-b border-muted pb-4">
      <div class="flex items-center justify-between gap-3"><code class="text-sm">{{ item.id }}</code><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('common.delete')" @click="remove(item.id)" /></div>
      <UFormField :label="t('nodeForms.providerType')"><USelect :model-value="item.type" :items="types" class="w-full" @update:model-value="updateType(item.id, String($event))" /></UFormField>
      <div class="grid gap-3 sm:grid-cols-2">
        <UFormField v-for="key in fields(item)" :key="key" :label="t('nodeForms.providerCredential', { name: key })">
          <UInput :model-value="item.credentials?.[key] ?? ''" type="password" autocomplete="new-password" :placeholder="t('nodeForms.secretUnchanged')" class="w-full" @update:model-value="updateSecret(item.id, key, String($event))" />
        </UFormField>
      </div>
    </div>
    <p v-if="!entries.length" class="text-sm text-muted">{{ t('nodeForms.noProviders') }}</p>
    <UButton class="justify-self-start" variant="outline" color="neutral" icon="i-ph-plus" :label="t('nodeForms.addProvider')" @click="add" />
  </section>
</template>
