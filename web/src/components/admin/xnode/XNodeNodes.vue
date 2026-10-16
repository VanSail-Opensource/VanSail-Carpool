<script setup lang="ts">
import { computed, reactive, ref, shallowRef } from 'vue'
import { xnodeApi, type XNode, type XNodeConfiguration, type XNodeSettings, type XNodeMetrics, type XNodeEnrollment, type XNodeEndpoint } from '@/api/xnode'
import { localizedError, useI18n } from '@/i18n'
import XNodeOperationalSettings from './XNodeOperationalSettings.vue'
import XNodeCommands from './XNodeCommands.vue'

const props = defineProps<{ nodes: XNode[]; endpoints: XNodeEndpoint[]; configurations: XNodeConfiguration[]; metrics: XNodeMetrics | null; busy: boolean; saveError?: string | null; perform: (action: () => Promise<unknown>) => Promise<boolean> }>()
const { t } = useI18n()
const editing = shallowRef(false)
const loading = shallowRef(false)
const settingsValid = shallowRef(true)
const error = shallowRef<string | null>(null)
const enrollment = shallowRef<XNodeEnrollment | null>(null)
const commanding = shallowRef<XNode | null>(null)
const draft = reactive<Partial<XNode>>({})
const settings = ref<XNodeSettings>({ ddns: [], certificates: {}, inboundCertificates: {}, dns: {}, firewall: { enabled: false, protect: false } })
const configurations = computed(() => props.configurations.map(value => ({ label: value.name, value: value.id })))
const status = (id: string) => props.metrics?.nodes.find(value => value.nodeId === id)
const inbounds = computed(() => {
  const document = props.configurations.find(item => item.id === draft.configId)?.document
  if (!Array.isArray(document?.inbounds)) return []
  return document.inbounds.flatMap(item => item && typeof item === 'object' && typeof item.tag === 'string' && !['tunnel', 'dokodemo-door'].includes(item.protocol) ? [item.tag] : [])
})
function closeEnrollment(value: boolean): void { if (!value) enrollment.value = null }
async function edit(node?: XNode): Promise<void> {
  for (const key of Object.keys(draft)) delete draft[key as keyof XNode]
  settingsValid.value = true
  Object.assign(draft, { id: '', name: '', enabled: true, countryCode: '', providerName: '', multiplier: 1, configId: props.configurations[0]?.id ?? '' }, node ?? {})
  settings.value = { ddns: [], certificates: {}, inboundCertificates: {}, dns: {}, firewall: { enabled: false, protect: false } }
  enrollment.value = null; error.value = null; editing.value = true
  if (!node) return
  loading.value = true
  try { settings.value = (await xnodeApi.node(node.id)).settings }
  catch (caught) { error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { loading.value = false }
}
async function save(): Promise<void> {
  if (loading.value || error.value || !settingsValid.value) return
  if (await props.perform(() => xnodeApi.saveNode(draft, settings.value))) editing.value = false
}
async function enroll(node: XNode): Promise<void> {
  error.value = null
  try { enrollment.value = await xnodeApi.enroll(node.id) }
  catch (caught) { error.value = localizedError(caught, 'xnode.saveFailed') }
}
</script>

<template>
  <div class="xnode-list">
    <div class="flex justify-end"><UButton icon="i-ph-plus-bold" :disabled="!configurations.length" :label="t('xnode.addNode')" @click="edit()" /></div>
    <UAlert v-if="!configurations.length" color="neutral" variant="soft" :title="t('xnode.configurationFirst')" />
    <UAlert v-if="error" color="warning" variant="soft" :title="error" />
    <p v-if="!nodes.length" class="text-muted">{{ t('xnode.noNodes') }}</p>
    <div v-for="node in nodes" :key="node.id" class="xnode-row">
      <div><strong>{{ node.name }}</strong><div class="text-xs text-muted">{{ node.countryCode }} · {{ node.multiplier }}×</div></div>
      <UBadge :color="status(node.id)?.connected && !status(node.id)?.stale ? 'success' : 'neutral'" variant="subtle">{{ t(status(node.id)?.connected && !status(node.id)?.stale ? 'xnode.online' : 'xnode.offline') }}</UBadge>
      <div class="flex gap-2"><UButton variant="ghost" color="neutral" icon="i-ph-terminal-window" :aria-label="t('nodeAdmin.command')" @click="commanding = node" /><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :aria-label="t('common.edit')" @click="edit(node)" /><UButton variant="ghost" color="neutral" icon="i-ph-key-bold" :aria-label="t('xnode.enroll')" @click="enroll(node)" /></div>
    </div>
    <UModal v-model:open="editing" :title="t(draft.id ? 'xnode.editNode' : 'xnode.addNode')" :ui="{ content: 'max-w-4xl' }">
      <template #body>
        <UProgress v-if="loading" />
        <form v-else class="xnode-form" @submit.prevent="save">
          <UAlert v-if="saveError" color="warning" :title="saveError" />
          <div class="xnode-fields">
            <UFormField :label="t('common.name')"><UInput v-model="draft.name" required class="w-full" /></UFormField>
            <UFormField :label="t('xnode.configuration')"><USelect v-model="draft.configId" :items="configurations" class="w-full" /></UFormField>
            <UFormField :label="t('xnode.country')"><UInput v-model="draft.countryCode" maxlength="2" class="w-full" /></UFormField>
            <UFormField :label="t('xnode.provider')"><UInput v-model="draft.providerName" class="w-full" /></UFormField>
            <UFormField :label="t('xnode.multiplier')"><UInput v-model.number="draft.multiplier" type="number" min="0.000001" max="1000" step="0.01" class="w-full" /></UFormField>
            <UFormField><USwitch v-model="draft.enabled" :label="t('common.enabled')" /></UFormField>
          </div>
          <XNodeOperationalSettings v-model="settings" :inbounds="inbounds" @valid="settingsValid = $event" />
          <div class="flex justify-end"><UButton type="submit" :loading="busy" :disabled="!settingsValid" :label="t('common.save')" /></div>
        </form>
      </template>
    </UModal>
    <UModal :open="!!commanding" :title="t('nodeAdmin.command')" @update:open="!$event && (commanding = null)"><template #body><XNodeCommands v-if="commanding" :key="commanding.id" :node-id="commanding.id" :endpoints="endpoints" /></template></UModal>
    <UModal :open="!!enrollment" :title="t('xnode.enroll')" :description="t('xnode.enrollmentHint')" @update:open="closeEnrollment">
      <template #body><div v-if="enrollment" class="grid gap-4"><UFormField :label="t('xnode.masterUrl')"><UInput :model-value="enrollment.masterUrl" readonly class="w-full" /></UFormField><UFormField :label="t('xnode.enrollmentToken')"><UInput :model-value="enrollment.token" readonly class="w-full font-mono" /></UFormField></div></template>
    </UModal>
  </div>
</template>

<style scoped>
.xnode-list, .xnode-form { display: grid; gap: 16px; }
.xnode-row { display: grid; grid-template-columns: minmax(0,1fr) auto auto; gap: 12px; align-items: center; padding-block: 12px; border-bottom: 1px solid var(--line); }
.xnode-fields { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 16px; }
@media (max-width: 600px) { .xnode-fields { grid-template-columns: minmax(0,1fr); } }
</style>
