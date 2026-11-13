<script setup lang="ts">
import { computed, onScopeDispose, reactive, shallowRef, toRaw } from 'vue'
import { xnodeApi, type XNodeGroup, type XNodeEndpoint, type XNode, type XNodeConfiguration } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { cloneForEditing } from '@/utils/browserCompatibility'
import XNodeAccessBindings from './XNodeAccessBindings.vue'
import XNodeEndpointForm from './XNodeEndpointForm.vue'

const props = defineProps<{ groups: XNodeGroup[]; endpoints: XNodeEndpoint[]; nodes: XNode[]; configurations: XNodeConfiguration[]; busy: boolean; saveError?: string | null; perform: (action: () => Promise<unknown>) => Promise<boolean> }>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const groupOpen = shallowRef(false)
const endpointOpen = shallowRef(false)
const group = reactive({ id: '', name: '', enabled: true })
const endpoint = shallowRef<XNodeEndpoint | undefined>()
const endpointGeneration = shallowRef(0)
const bindings = shallowRef<XNodeGroup['bindings']>([])
const bindingValid = shallowRef(true)
const nodes = computed(() => props.nodes.map(item => ({ label: item.name, value: item.id })))
const submitting = shallowRef(false)
const blocked = computed(() => props.busy || submitting.value)
let active = true
onScopeDispose(() => { active = false })
function editGroup(item?: XNodeGroup): void {
  if (!active || blocked.value) return
  Object.assign(group, { id: item?.id ?? '', name: item?.name ?? '', enabled: item?.enabled ?? true })
  bindings.value = cloneForEditing(toRaw(item?.bindings ?? []))
  bindingValid.value = true
  groupOpen.value = true
}
function editEndpoint(item?: XNodeEndpoint): void {
  if (!active || blocked.value) return
  endpoint.value = item
  endpointGeneration.value++
  endpointOpen.value = true
}
async function performMutation(action: () => Promise<unknown>): Promise<boolean> {
  if (!active || blocked.value) return false
  submitting.value = true
  try { return await props.perform(action) }
  finally { if (active) submitting.value = false }
}
async function saveGroup(): Promise<void> {
  if (!bindingValid.value || blocked.value) return
  const payload = { ...group, bindings: cloneForEditing(toRaw(bindings.value)) }
  if (await performMutation(() => xnodeApi.saveGroup(payload)) && active) groupOpen.value = false
}
async function deleteEndpoint(item: XNodeEndpoint): Promise<void> {
  const id = item.id
  await performMutation(() => xnodeApi.deleteEndpoint(id))
}
async function deleteGroup(item: XNodeGroup): Promise<void> {
  if (!item.canDelete || item.memberCount !== 0 || blocked.value) return
  const id = item.id
  if (!await performMutation(() => xnodeApi.deleteGroup(id)) && active) emit('refresh')
}
</script>

<template>
  <div class="grid gap-5">
    <div class="flex justify-end gap-2"><UButton icon="i-ph-plus-bold" :disabled="blocked" :label="t('xnode.addGroup')" @click="editGroup()" /><UButton color="neutral" variant="outline" icon="i-ph-globe-bold" :disabled="blocked || !nodes.length" :label="t('xnode.addEndpoint')" @click="editEndpoint()" /></div>
    <p v-if="!groups.length" class="text-muted">{{ t('xnode.noGroups') }}</p>
    <div v-for="item in groups" :key="item.id" class="resource-row"><strong>{{ item.name }}</strong><span class="text-muted text-xs">{{ item.bindings.length }}</span><div class="flex gap-1"><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :disabled="blocked" :aria-label="t('common.edit')" @click="editGroup(item)" /><UButton v-if="item.canDelete && item.memberCount === 0" variant="ghost" color="error" icon="i-ph-trash-bold" :disabled="blocked" :aria-label="t('xnode.deleteGroup', { name: item.name })" @click="deleteGroup(item)" /></div></div>
    <h3>{{ t('xnode.endpoints') }}</h3>
    <p v-if="!endpoints.length" class="text-muted">{{ t('xnode.noEndpoints') }}</p>
    <div v-for="item in endpoints" :key="item.id" class="resource-row"><div><strong>{{ item.name }}</strong><div class="text-xs text-muted break-all">{{ item.address }}:{{ item.port }} · {{ item.inboundTag }}</div><div v-if="item.metadata?.protocol" class="text-xs text-muted">{{ t('endpointOptions.metadata', { protocol: item.metadata.protocol, transport: item.metadata.transport || t('endpointOptions.default'), security: item.metadata.security || t('endpointOptions.none') }) }}</div></div><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :disabled="blocked" :aria-label="t('common.edit')" @click="editEndpoint(item)" /><UButton variant="ghost" color="error" icon="i-ph-trash-bold" :disabled="blocked" :aria-label="t('xnode.deleteEndpoint', { name: item.name })" @click="deleteEndpoint(item)" /></div>
    <UModal :open="groupOpen" :dismissible="!blocked" :close="!blocked" @update:open="!blocked && (groupOpen = $event)" :title="t('xnode.group')"><template #body><form class="grid gap-4" @submit.prevent="saveGroup"><UAlert v-if="saveError" color="warning" :title="saveError" /><UFormField :label="t('common.name')"><UInput v-model="group.name" :disabled="blocked" required class="w-full" /></UFormField><USwitch v-model="group.enabled" :disabled="blocked" :label="t('common.enabled')" /><XNodeAccessBindings v-model="bindings" :disabled="blocked" :nodes="props.nodes" :configurations="configurations" @valid="bindingValid = $event" /><UButton type="submit" :loading="blocked" :disabled="blocked || !bindingValid" :label="t('common.save')" class="justify-self-end" /></form></template></UModal>
    <UModal :open="endpointOpen" :dismissible="!blocked" :close="!blocked" @update:open="!blocked && (endpointOpen = $event)" :title="t('xnode.endpoint')" :ui="{ content: 'sm:max-w-4xl' }"><template #body><XNodeEndpointForm :key="endpointGeneration" :item="endpoint" :nodes="props.nodes" :configurations="configurations" :busy="blocked" :save-error="saveError" :perform="performMutation" @saved="endpointOpen = false" /></template></UModal>
  </div>
</template>

<style scoped>
.resource-row { display: grid; grid-template-columns: minmax(0,1fr) auto auto; gap: 12px; align-items: center; padding-block: 10px; border-bottom: 1px solid var(--line); }
h3 { margin: 8px 0 0; font-size: 15px; }
</style>
