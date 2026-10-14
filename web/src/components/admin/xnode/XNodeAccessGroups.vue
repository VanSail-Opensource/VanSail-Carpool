<script setup lang="ts">
import { computed, reactive, shallowRef } from 'vue'
import { xnodeApi, type XNodeGroup, type XNodeEndpoint, type XNode } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeJsonEditor from './XNodeJsonEditor.vue'

const props = defineProps<{ groups: XNodeGroup[]; endpoints: XNodeEndpoint[]; nodes: XNode[]; busy: boolean; perform: (action: () => Promise<unknown>) => Promise<boolean> }>()
const { t } = useI18n()
const groupOpen = shallowRef(false)
const endpointOpen = shallowRef(false)
const group = reactive<Partial<XNodeGroup>>({})
const endpoint = reactive<Partial<XNodeEndpoint>>({})
const bindings = shallowRef<unknown>([])
const bindingValid = shallowRef(true)
const nodes = computed(() => props.nodes.map(item => ({ label: item.name, value: item.id })))
function editGroup(item?: XNodeGroup): void { Object.assign(group, { id: '', name: '', enabled: true }, item ?? {}); bindings.value = item?.bindings ?? []; bindingValid.value = true; groupOpen.value = true }
function editEndpoint(item?: XNodeEndpoint): void { Object.assign(endpoint, { id: '', nodeId: props.nodes[0]?.id ?? '', inboundTag: '', name: '', address: '', port: 443, enabled: true, hidden: false, overrides: {} }, item ?? {}); endpointOpen.value = true }
async function saveGroup(): Promise<void> { if (bindingValid.value && await props.perform(() => xnodeApi.saveGroup({ ...group, bindings: bindings.value as XNodeGroup['bindings'] }))) groupOpen.value = false }
async function saveEndpoint(): Promise<void> { if (await props.perform(() => xnodeApi.saveEndpoint(endpoint))) endpointOpen.value = false }
</script>

<template>
  <div class="grid gap-5">
    <div class="flex justify-end gap-2"><UButton icon="i-ph-plus-bold" :label="t('xnode.addGroup')" @click="editGroup()" /><UButton color="neutral" variant="outline" icon="i-ph-globe-bold" :disabled="!nodes.length" :label="t('xnode.addEndpoint')" @click="editEndpoint()" /></div>
    <p v-if="!groups.length" class="text-muted">{{ t('xnode.noGroups') }}</p>
    <div v-for="item in groups" :key="item.id" class="resource-row"><strong>{{ item.name }}</strong><span class="text-muted text-xs">{{ item.bindings.length }}</span><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :aria-label="t('common.edit')" @click="editGroup(item)" /></div>
    <h3>{{ t('xnode.endpoints') }}</h3>
    <div v-for="item in endpoints" :key="item.id" class="resource-row"><div><strong>{{ item.name }}</strong><div class="text-xs text-muted">{{ item.address }}:{{ item.port }} · {{ item.inboundTag }}</div></div><span /><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :aria-label="t('common.edit')" @click="editEndpoint(item)" /></div>
    <UModal v-model:open="groupOpen" :title="t('xnode.group')"><template #body><form class="grid gap-4" @submit.prevent="saveGroup"><UFormField :label="t('common.name')"><UInput v-model="group.name" required class="w-full" /></UFormField><USwitch v-model="group.enabled" :label="t('common.enabled')" /><XNodeJsonEditor v-model="bindings" :label="t('xnode.groupBindings')" :rows="8" @valid="bindingValid = $event" /><UButton type="submit" :loading="busy" :disabled="!bindingValid" :label="t('common.save')" class="justify-self-end" /></form></template></UModal>
    <UModal v-model:open="endpointOpen" :title="t('xnode.endpoint')"><template #body><form class="grid gap-4" @submit.prevent="saveEndpoint"><UFormField :label="t('common.name')"><UInput v-model="endpoint.name" required class="w-full" /></UFormField><UFormField :label="t('xnode.node')"><USelect v-model="endpoint.nodeId" :items="nodes" class="w-full" /></UFormField><UFormField :label="t('xnode.inboundTag')"><UInput v-model="endpoint.inboundTag" required class="w-full" /></UFormField><UFormField :label="t('xnode.address')"><UInput v-model="endpoint.address" required class="w-full" /></UFormField><UFormField :label="t('xnode.port')"><UInput v-model.number="endpoint.port" type="number" min="1" max="65535" class="w-full" /></UFormField><USwitch v-model="endpoint.enabled" :label="t('common.enabled')" /><USwitch v-model="endpoint.hidden" :label="t('xnode.hiddenEndpoint')" /><UButton type="submit" :loading="busy" :label="t('common.save')" class="justify-self-end" /></form></template></UModal>
  </div>
</template>

<style scoped>
.resource-row { display: grid; grid-template-columns: minmax(0,1fr) auto auto; gap: 12px; align-items: center; padding-block: 10px; border-bottom: 1px solid var(--line); }
h3 { margin: 8px 0 0; font-size: 15px; }
</style>
