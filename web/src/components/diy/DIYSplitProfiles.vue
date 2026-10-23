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
const downloadIds = (id: string) => host(id)?.downloadOptions?.length ? host(id)!.downloadOptions!.map(option => option.downloadEndpointId) : host(id)?.downloadIds ?? []
const pairModes = (uploadId: string, downloadId: string) => host(uploadId)?.downloadOptions?.find(option => option.downloadEndpointId === downloadId)?.modes ?? host(uploadId)?.modes ?? []
const defaultMode = (uploadId: string, downloadId: string): DIYSplitProfile['mode'] => pairModes(uploadId, downloadId)[0] ?? 'auto'
const pairs = computed(() => props.hosts.flatMap(upload => downloadIds(upload.id).map(downloadEndpointId => ({ uploadEndpointId: upload.id, downloadEndpointId }))))
const downloads = (id: string) => downloadIds(id).map(value => ({ label: host(value)?.name ?? value, value }))
const modes = (uploadId: string, downloadId: string) => pairModes(uploadId, downloadId).map(value => ({ label: value, value }))
function add() {
  const pair = pairs.value.find(candidate => !model.value.some(profile => profile.uploadEndpointId === candidate.uploadEndpointId && profile.downloadEndpointId === candidate.downloadEndpointId))
  if (pair) model.value = [...model.value, { id: createUuid(), ...pair, mode: defaultMode(pair.uploadEndpointId, pair.downloadEndpointId) }]
}
function setUpload(profile: DIYSplitProfile, id: string) {
  const downloadEndpointId = downloadIds(id)[0] ?? ''
  model.value = model.value.map(item => item.id !== profile.id ? item : { ...item, uploadEndpointId: id, downloadEndpointId, mode: defaultMode(id, downloadEndpointId) })
}
function setDownload(profile: DIYSplitProfile, downloadEndpointId: string) {
  model.value = model.value.map(item => item.id !== profile.id ? item : { ...item, downloadEndpointId, mode: defaultMode(profile.uploadEndpointId, downloadEndpointId) })
}
function update(id: string, patch: Partial<DIYSplitProfile>) { model.value = model.value.map(item => item.id === id ? { ...item, ...patch } : item) }
</script>

<template>
  <section class="grid gap-3 min-w-0">
    <UAlert v-if="error" color="warning" :title="error" />
    <p v-if="!hosts.length" class="text-sm text-muted">{{ t('diy.split.unavailable') }}</p>
    <p v-else-if="!model.length" class="text-sm text-muted">{{ t('diy.split.empty') }}</p>
    <fieldset v-for="(profile, index) in model" :key="profile.id" :disabled="disabled" class="grid grid-cols-[minmax(0,1fr)_auto] sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_8rem_auto] gap-2 min-w-0 border-b border-default pb-3">
      <legend class="sr-only">{{ t('diy.split.number', { number: index + 1 }) }}</legend>
      <UFormField class="col-span-2 sm:col-span-1 min-w-0" :label="t('diy.split.upload')"><USelect :model-value="profile.uploadEndpointId" :items="uploads" class="w-full" @update:model-value="setUpload(profile, $event)" /></UFormField>
      <UFormField class="col-span-2 sm:col-span-1 min-w-0" :label="t('diy.split.download')"><USelect :model-value="profile.downloadEndpointId" :items="downloads(profile.uploadEndpointId)" class="w-full" @update:model-value="setDownload(profile, $event)" /></UFormField>
      <UFormField class="min-w-0" :label="t('diy.split.mode')"><USelect :model-value="profile.mode" :items="modes(profile.uploadEndpointId, profile.downloadEndpointId)" class="w-full" @update:model-value="update(profile.id, { mode: $event })" /></UFormField>
      <UButton data-haptic="action" class="self-end justify-self-end" variant="ghost" color="neutral" icon="i-ph-trash-bold" :aria-label="t('diy.split.remove', { number: index + 1 })" :disabled="disabled" @click="model = model.filter(item => item.id !== profile.id)" />
    </fieldset>
    <UButton data-haptic="action" class="justify-self-start" variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('diy.split.add')" :disabled="disabled || !hosts.length || model.length >= 64" @click="add" />
  </section>
</template>
