<script setup lang="ts">
import { onMounted, onScopeDispose, shallowRef } from 'vue'
import { xnodeApi, type NamedService, type NamedServiceWrite, type XNode, type XNodeGroup } from '@/api/xnode'
import { localizedError, useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import XNodeServiceEditor from './XNodeServiceEditor.vue'
import XNodeServiceRouting from './XNodeServiceRouting.vue'

defineProps<{ nodes: XNode[]; groups: XNodeGroup[] }>()
const { t } = useI18n()
const items = shallowRef<NamedService[]>([])
const editing = shallowRef<NamedService | null>(null)
const routing = shallowRef<NamedService | null>(null)
const open = shallowRef(false)
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const link = shallowRef('')
let active = true
async function load() {
  loading.value = true; error.value = null
  try { const result = await xnodeApi.namedServices(); if (active) items.value = result.items }
  catch (caught) { if (active) error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { if (active) loading.value = false }
}
async function save(body: NamedServiceWrite) {
  if (busy.value) return
  busy.value = true; error.value = null
  try { await xnodeApi.saveNamedService(editing.value?.identity.proxyUserId, body); open.value = false; await load() }
  catch (caught) { error.value = localizedError(caught, 'xnode.saveFailed') }
  finally { busy.value = false }
}
async function subscription(item: NamedService, rotate = false) {
  if (busy.value) return
  busy.value = true; error.value = null; link.value = ''
  try { link.value = rotate ? (await xnodeApi.rotateServiceSubscription(item.identity.proxyUserId)).url : (await xnodeApi.serviceSubscription(item.identity.proxyUserId)).url }
  catch (caught) { error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { busy.value = false }
}
onMounted(load)
onScopeDispose(() => { active = false })
</script>

<template>
  <div class="grid gap-4 min-w-0">
    <div class="flex justify-between gap-3"><p class="text-sm text-muted">{{ t('namedServices.description') }}</p><div class="flex gap-1"><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :disabled="busy" :loading="loading" @click="load" /><UButton icon="i-ph-plus-bold" :label="t('namedServices.add')" :disabled="busy || !groups.length" @click="editing = null; open = true" /></div></div>
    <UAlert v-if="error" color="warning" :title="error" />
    <UProgress v-if="loading" />
    <div v-else-if="!items.length && !error" class="flex items-center justify-between gap-3"><p class="text-muted">{{ t('namedServices.empty') }}</p><UButton variant="ghost" :label="t('common.refresh')" @click="load" /></div>
    <div v-for="item in items" :key="item.identity.proxyUserId" class="flex flex-wrap justify-between items-center gap-3 py-3 border-b border-default">
      <div><strong>{{ item.identity.serviceName }}</strong><p class="text-xs text-muted">{{ formatDateTime(item.access.expiresAt) }} · {{ t(item.access.disabled ? 'namedServices.disabled' : 'common.enabled') }}</p></div>
      <div class="flex flex-wrap gap-1">
        <UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :aria-label="t('common.edit')" :disabled="busy" @click="editing = item; open = true" />
        <UButton variant="ghost" color="neutral" icon="i-ph-git-branch-bold" :aria-label="t('memberNodes.routing')" :disabled="busy" @click="routing = item" />
        <UButton variant="ghost" color="neutral" icon="i-ph-link-bold" :aria-label="t('namedServices.subscription')" :disabled="busy" @click="subscription(item)" />
        <UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise-bold" :aria-label="t('namedServices.rotate')" :disabled="busy" @click="subscription(item, true)" />
      </div>
    </div>
    <UFormField v-if="link" :label="t('namedServices.subscription')"><UInput :model-value="link" readonly class="w-full font-mono" /></UFormField>
    <XNodeServiceEditor v-if="open" :item="editing" :error="error" :groups="groups" :busy="busy" @close="open = false" @save="save" />
    <XNodeServiceRouting v-if="routing" :key="routing.identity.proxyUserId" :item="routing" :nodes="nodes" :groups="groups" @close="routing = null" />
  </div>
</template>
