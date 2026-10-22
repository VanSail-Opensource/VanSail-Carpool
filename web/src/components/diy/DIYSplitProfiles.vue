<script setup lang="ts">
import { computed } from 'vue'
import type { DIYSplitProfile, DIYSplitHost } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

const model = defineModel<DIYSplitProfile[]>({ required: true })
const props = defineProps<{ hosts: DIYSplitHost[]; disabled?: boolean; error?: string }>()
const { t } = useI18n()
const uploads = computed(() => props.hosts.map(host => ({ label: host.name, value: host.id })))
const host = (id: string) => props.hosts.find(item => item.id === id)
const downloads = (id: string) => props.hosts.filter(item => host(id)?.downloadIds.includes(item.id)).map(item => ({ label: item.name, value: item.id }))
const modes = (id: string) => (host(id)?.modes ?? []).map(value => ({ label: value, value }))
function add() {
  const up = props.hosts.find(item => item.downloadIds.some(down => !model.value.some(profile => profile.uploadEndpointId === item.id && profile.downloadEndpointId === down)))
  const down = up?.downloadIds.find(id => !model.value.some(profile => profile.uploadEndpointId === up.id && profile.downloadEndpointId === id))
  if (up && down) model.value = [...model.value, { id: createUuid(), uploadEndpointId: up.id, downloadEndpointId: down, mode: up.modes[0] ?? 'auto' }]
}
function setUpload(profile: DIYSplitProfile, id: string) {
  const next = host(id)
  model.value = model.value.map(item => item.id !== profile.id ? item : { ...item, uploadEndpointId: id, downloadEndpointId: next?.downloadIds[0] ?? '', mode: next?.modes[0] ?? 'auto' })
}
function update(id: string, patch: Partial<DIYSplitProfile>) { model.value = model.value.map(item => item.id === id ? { ...item, ...patch } : item) }
</script>

<template>
  <section class="grid gap-3 min-w-0">
    <div><h2 class="text-base font-medium">{{ t('diy.split.title') }}</h2><p class="text-xs text-muted">{{ t('diy.split.hint') }}</p></div>
    <UAlert v-if="error" color="warning" :title="error" />
    <p v-if="!hosts.length" class="text-sm text-muted">{{ t('diy.split.unavailable') }}</p>
    <p v-else-if="!model.length" class="text-sm text-muted">{{ t('diy.split.empty') }}</p>
    <fieldset v-for="(profile, index) in model" :key="profile.id" :disabled="disabled" class="grid grid-cols-1 sm:grid-cols-[1fr_1fr_8rem_auto] gap-2 min-w-0 border-b border-default pb-3">
      <legend class="sr-only">{{ t('diy.split.number', { number: index + 1 }) }}</legend>
      <UFormField :label="t('diy.split.upload')"><USelect :model-value="profile.uploadEndpointId" :items="uploads" class="w-full" @update:model-value="setUpload(profile, $event)" /></UFormField>
      <UFormField :label="t('diy.split.download')"><USelect :model-value="profile.downloadEndpointId" :items="downloads(profile.uploadEndpointId)" class="w-full" @update:model-value="update(profile.id, { downloadEndpointId: $event })" /></UFormField>
      <UFormField :label="t('diy.split.mode')"><USelect :model-value="profile.mode" :items="modes(profile.uploadEndpointId)" class="w-full" @update:model-value="update(profile.id, { mode: $event })" /></UFormField>
      <UButton class="self-end justify-self-end" variant="ghost" color="neutral" icon="i-ph-trash-bold" :aria-label="t('diy.split.remove', { number: index + 1 })" :disabled="disabled" @click="model = model.filter(item => item.id !== profile.id)" />
    </fieldset>
    <UButton class="justify-self-start" variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('diy.split.add')" :disabled="disabled || !hosts.length || model.length >= 64" @click="add" />
  </section>
</template>
