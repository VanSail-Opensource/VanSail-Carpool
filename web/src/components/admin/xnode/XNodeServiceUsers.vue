<script setup lang="ts">
import { onMounted, onScopeDispose, shallowRef, toRaw } from 'vue'
import { xnodeApi, type NamedService, type NamedServiceWrite, type XNode, type XNodeGroup } from '@/api/xnode'
import { localizedError, useI18n } from '@/i18n'
import { cloneForEditing } from '@/utils/browserCompatibility'
import { formatDateTime } from '@/utils/format'
import { createLatestRequest } from '@/utils/latestRequest'
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
const linkOwner = shallowRef('')
const reads = createLatestRequest()
const writes = createLatestRequest()
let active = true
function edit(item: NamedService | null): void {
  if (!active || busy.value) return
  editing.value = item
  error.value = null
  open.value = true
}
async function load() {
  if (!active) return
  const generation = reads.begin()
  loading.value = true
  error.value = null
  try {
    const result = await xnodeApi.namedServices()
    if (reads.isCurrent(generation)) items.value = result.items
  } catch (caught) {
    if (reads.isCurrent(generation)) error.value = localizedError(caught, 'xnode.loadFailed')
  } finally {
    if (reads.isCurrent(generation)) loading.value = false
  }
}
async function save(body: NamedServiceWrite) {
  if (!active || busy.value) return
  const owner = editing.value?.identity.proxyUserId
  const payload = cloneForEditing(toRaw(body))
  const generation = writes.begin()
  reads.invalidate()
  loading.value = false
  busy.value = true
  error.value = null
  try {
    await xnodeApi.saveNamedService(owner, payload)
    if (!writes.isCurrent(generation)) return
    open.value = false
    await load()
  } catch (caught) {
    if (writes.isCurrent(generation)) error.value = localizedError(caught, 'xnode.saveFailed')
  } finally {
    if (writes.isCurrent(generation)) busy.value = false
  }
}
async function subscription(item: NamedService, rotate = false) {
  if (!active || busy.value) return
  const owner = item.identity.proxyUserId
  const name = item.identity.serviceName
  const generation = writes.begin()
  reads.invalidate()
  loading.value = false
  busy.value = true
  error.value = null
  link.value = ''
  try {
    const result = rotate ? await xnodeApi.rotateServiceSubscription(owner) : await xnodeApi.serviceSubscription(owner)
    if (!writes.isCurrent(generation)) return
    linkOwner.value = name
    link.value = result.url
  } catch (caught) {
    if (writes.isCurrent(generation)) error.value = localizedError(caught, 'xnode.loadFailed')
  } finally {
    if (writes.isCurrent(generation)) busy.value = false
  }
}
onMounted(load)
onScopeDispose(() => { active = false; reads.dispose(); writes.dispose(); link.value = '' })
</script>

<template>
  <div class="grid gap-4 min-w-0">
    <div class="flex justify-between gap-3"><p class="text-sm text-muted">{{ t('namedServices.description') }}</p><div class="flex gap-1"><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :disabled="busy" :loading="loading" @click="load" /><UButton icon="i-ph-plus-bold" :label="t('namedServices.add')" :disabled="busy || !groups.length" @click="edit(null)" /></div></div>
    <UAlert v-if="error" color="warning" :title="error" />
    <UProgress v-if="loading" />
    <div v-else-if="!items.length && !error" class="flex items-center justify-between gap-3"><p class="text-muted">{{ t('namedServices.empty') }}</p><UButton variant="ghost" :label="t('common.refresh')" @click="load" /></div>
    <div v-for="item in items" :key="item.identity.proxyUserId" class="flex flex-wrap justify-between items-center gap-3 py-3 border-b border-default">
      <div><strong>{{ item.identity.serviceName }}</strong><p class="text-xs text-muted">{{ formatDateTime(item.access.expiresAt) }} · {{ t(item.access.disabled ? 'namedServices.disabled' : 'common.enabled') }}</p></div>
      <div class="flex flex-wrap gap-1">
        <UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :aria-label="t('common.edit')" :disabled="busy" @click="edit(item)" />
        <UButton variant="ghost" color="neutral" icon="i-ph-git-branch-bold" :aria-label="t('memberNodes.routing')" :disabled="busy" @click="!busy && (routing = item)" />
        <UButton variant="ghost" color="neutral" icon="i-ph-link-bold" :aria-label="t('namedServices.subscription')" :disabled="busy" @click="subscription(item)" />
        <UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise-bold" :aria-label="t('namedServices.rotate')" :disabled="busy" @click="subscription(item, true)" />
      </div>
    </div>
    <UFormField v-if="link" :label="t('namedServices.subscriptionFor', { name: linkOwner })"><UInput :model-value="link" readonly class="w-full font-mono" /></UFormField>
    <XNodeServiceEditor v-if="open" :key="editing?.identity.proxyUserId ?? 'new'" :item="editing" :error="error" :groups="groups" :busy="busy" @close="!busy && (open = false)" @save="save" />
    <XNodeServiceRouting v-if="routing" :key="routing.identity.proxyUserId" :item="routing" :nodes="nodes" :groups="groups" @close="routing = null" />
  </div>
</template>
