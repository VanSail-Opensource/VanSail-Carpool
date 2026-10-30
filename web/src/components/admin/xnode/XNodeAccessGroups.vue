<script setup lang="ts">
import { computed, reactive, shallowRef } from 'vue'
import { xnodeApi, type XNodeGroup, type XNodeEndpoint, type XNode, type XNodeConfiguration } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeAccessBindings from './XNodeAccessBindings.vue'
import XNodeEndpointOptions from './XNodeEndpointOptions.vue'
import { nodeInboundItems } from './nodeFormOptions'

const props = defineProps<{ groups: XNodeGroup[]; endpoints: XNodeEndpoint[]; nodes: XNode[]; configurations: XNodeConfiguration[]; busy: boolean; saveError?: string | null; perform: (action: () => Promise<unknown>) => Promise<boolean> }>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const groupOpen = shallowRef(false)
const endpointOpen = shallowRef(false)
const group = reactive<Partial<XNodeGroup>>({})
const endpoint = reactive<Partial<XNodeEndpoint>>({})
const bindings = shallowRef<XNodeGroup['bindings']>([])
const bindingValid = shallowRef(true)
const nodes = computed(() => props.nodes.map(item => ({ label: item.name, value: item.id })))
const inboundOptions = computed(() => nodeInboundItems(endpoint.nodeId, props.nodes, props.configurations))
function editGroup(item?: XNodeGroup): void { Object.assign(group, { id: '', name: '', enabled: true }, item ?? {}); bindings.value = globalThis.structuredClone(item?.bindings ?? []); bindingValid.value = true; groupOpen.value = true }
function editEndpoint(item?: XNodeEndpoint): void { Object.assign(endpoint, { id: '', nodeId: props.nodes[0]?.id ?? '', inboundTag: '', name: '', address: '', port: 443, enabled: true, hidden: false, overrides: {} }, item ?? {}); endpoint.overrides = globalThis.structuredClone(item?.overrides ?? {}); endpointOpen.value = true }
async function saveGroup(): Promise<void> { if (bindingValid.value && await props.perform(() => xnodeApi.saveGroup({ ...group, bindings: bindings.value }))) groupOpen.value = false }
async function saveEndpoint(): Promise<void> { if (await props.perform(() => xnodeApi.saveEndpoint(endpoint))) endpointOpen.value = false }
async function deleteEndpoint(item: XNodeEndpoint): Promise<void> { await props.perform(() => xnodeApi.deleteEndpoint(item.id)) }
async function deleteGroup(item: XNodeGroup): Promise<void> { if (item.canDelete && item.memberCount === 0 && !await props.perform(() => xnodeApi.deleteGroup(item.id))) emit('refresh') }
</script>

<template>
  <div class="grid gap-5">
    <div class="flex justify-end gap-2"><UButton icon="i-ph-plus-bold" :label="t('xnode.addGroup')" @click="editGroup()" /><UButton color="neutral" variant="outline" icon="i-ph-globe-bold" :disabled="!nodes.length" :label="t('xnode.addEndpoint')" @click="editEndpoint()" /></div>
    <p v-if="!groups.length" class="text-muted">{{ t('xnode.noGroups') }}</p>
    <div v-for="item in groups" :key="item.id" class="resource-row"><strong>{{ item.name }}</strong><span class="text-muted text-xs">{{ item.bindings.length }}</span><div class="flex gap-1"><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :disabled="busy" :aria-label="t('common.edit')" @click="editGroup(item)" /><UButton v-if="item.canDelete && item.memberCount === 0" variant="ghost" color="error" icon="i-ph-trash-bold" :disabled="busy" :aria-label="t('xnode.deleteGroup', { name: item.name })" @click="deleteGroup(item)" /></div></div>
    <h3>{{ t('xnode.endpoints') }}</h3>
    <p v-if="!endpoints.length" class="text-muted">{{ t('xnode.noEndpoints') }}</p>
    <div v-for="item in endpoints" :key="item.id" class="resource-row"><div><strong>{{ item.name }}</strong><div class="text-xs text-muted break-all">{{ item.address }}:{{ item.port }} · {{ item.inboundTag }}</div></div><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :disabled="busy" :aria-label="t('common.edit')" @click="editEndpoint(item)" /><UButton variant="ghost" color="error" icon="i-ph-trash-bold" :disabled="busy" :aria-label="t('xnode.deleteEndpoint', { name: item.name })" @click="deleteEndpoint(item)" /></div>
    <UModal v-model:open="groupOpen" :title="t('xnode.group')"><template #body><form class="grid gap-4" @submit.prevent="saveGroup"><UAlert v-if="saveError" color="warning" :title="saveError" /><UFormField :label="t('common.name')"><UInput v-model="group.name" required class="w-full" /></UFormField><USwitch v-model="group.enabled" :label="t('common.enabled')" /><XNodeAccessBindings v-model="bindings" :nodes="props.nodes" :configurations="configurations" @valid="bindingValid = $event" /><UButton type="submit" :loading="busy" :disabled="!bindingValid" :label="t('common.save')" class="justify-self-end" /></form></template></UModal>
    <UModal v-model:open="endpointOpen" :title="t('xnode.endpoint')"><template #body><form class="grid gap-4" @submit.prevent="saveEndpoint"><UAlert v-if="saveError" color="warning" :title="saveError" /><UFormField :label="t('common.name')"><UInput v-model="endpoint.name" required class="w-full" /></UFormField><UFormField :label="t('xnode.node')"><USelect v-model="endpoint.nodeId" :items="nodes" class="w-full" /></UFormField><UFormField :label="t('xnode.inboundTag')"><USelect v-model="endpoint.inboundTag" :items="inboundOptions" class="w-full" /></UFormField><UFormField :label="t('xnode.address')"><UInput v-model="endpoint.address" required class="w-full" /></UFormField><UFormField :label="t('xnode.port')"><UInput v-model.number="endpoint.port" type="number" min="1" max="65535" class="w-full" /></UFormField><USwitch v-model="endpoint.enabled" :label="t('common.enabled')" /><USwitch v-model="endpoint.hidden" :label="t('xnode.hiddenEndpoint')" /><XNodeEndpointOptions v-model="endpoint.overrides" /><UButton type="submit" :loading="busy" :disabled="!endpoint.nodeId || !endpoint.inboundTag" :label="t('common.save')" class="justify-self-end" /></form></template></UModal>
  </div>
</template>

<style scoped>
.resource-row { display: grid; grid-template-columns: minmax(0,1fr) auto auto; gap: 12px; align-items: center; padding-block: 10px; border-bottom: 1px solid var(--line); }
h3 { margin: 8px 0 0; font-size: 15px; }
</style>
