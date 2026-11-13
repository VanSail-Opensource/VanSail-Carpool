<script setup lang="ts">
import { computed, onScopeDispose, reactive, ref, shallowRef, toRaw, watch } from 'vue'
import { xnodeApi, type XNode, type XNodeWrite, type XNodeConfiguration, type XNodeSettings, type XNodeMetrics, type XNodeEnrollment, type XNodeEndpoint } from '@/api/xnode'
import { localizedError, useI18n } from '@/i18n'
import XNodeOperationalSettings from './XNodeOperationalSettings.vue'
import XNodePricingCostSettings from './XNodePricingCostSettings.vue'
import XNodeRemovalDialog from './XNodeRemovalDialog.vue'
import XNodeCommands from './XNodeCommands.vue'
import { createLatestRequest } from '@/utils/latestRequest'
import { cloneForEditing } from '@/utils/browserCompatibility'

const props = defineProps<{ nodes: XNode[]; endpoints: XNodeEndpoint[]; configurations: XNodeConfiguration[]; metrics: XNodeMetrics | null; busy: boolean; saveError?: string | null; perform: (action: () => Promise<unknown>) => Promise<boolean> }>()
const emit = defineEmits<{ refresh: [] }>()
const removing = shallowRef<XNode | null>(null)
const { t } = useI18n()
const editing = shallowRef(false)
const submitting = shallowRef(false)
const blocked = computed(() => props.busy || submitting.value)
let active = true
const loading = shallowRef(false)
const loaded = shallowRef(false)
const latest = createLatestRequest()
const enrollmentLatest = createLatestRequest()
const enrollingId = shallowRef('')
const enrollmentNode = shallowRef('')
let controller: InstanceType<typeof globalThis.AbortController> | undefined
watch(editing, open => { if (!open) { controller?.abort(); latest.invalidate(); loaded.value = false } })
onScopeDispose(() => { active = false; controller?.abort(); latest.dispose(); enrollmentLatest.dispose(); enrollment.value = null })
const settingsValid = shallowRef(true)
const error = shallowRef<string | null>(null)
const enrollment = shallowRef<XNodeEnrollment | null>(null)
const commanding = shallowRef<XNode | null>(null)
const draft = reactive<Partial<XNodeWrite>>({})
const settings = ref<XNodeSettings>({ ddns: [], certificates: {}, inboundCertificates: {}, dns: {}, firewall: { enabled: false, protect: false } })
const certificateBaseline = shallowRef(''), loadedConfiguration = shallowRef(''), nodeRevision = shallowRef(0)
const savedCertificateIds = shallowRef<string[]>([])
const certificatesSaved = computed(() => certificateBaseline.value === JSON.stringify([settings.value.certificates ?? {}, settings.value.inboundCertificates ?? {}]) && draft.configId === loadedConfiguration.value)
const configurations = computed(() => props.configurations.map(value => ({ label: value.name, value: value.id })))
const ddnsHosts = computed(() => props.endpoints.filter(host => host.nodeId === draft.id))
const status = (id: string) => props.metrics?.nodes.find(value => value.nodeId === id)
const inbounds = computed(() => {
  const document = props.configurations.find(item => item.id === draft.configId)?.document
  if (!Array.isArray(document?.inbounds)) return []
  return document.inbounds.flatMap(item => item && typeof item === 'object' && typeof item.tag === 'string' && !['tunnel', 'dokodemo-door'].includes(item.protocol) ? [item.tag] : [])
})
function closeEnrollment(value: boolean): void { if (!value) enrollment.value = null }
async function edit(node?: XNode): Promise<void> {
  if (!active || blocked.value) return
  enrollmentLatest.invalidate(); enrollingId.value = ''
  controller?.abort()
  controller = new globalThis.AbortController()
  const ticket = latest.begin()
  loading.value = false; loaded.value = !node
  for (const key of Object.keys(draft)) delete draft[key as keyof XNode]
  settingsValid.value = true
  Object.assign(draft, { id: '', name: '', enabled: true, countryCode: '', providerName: '', configId: props.configurations[0]?.id ?? '' }, node ?? {})
  delete draft.multiplier
  settings.value = { ddns: [], certificates: {}, inboundCertificates: {}, dns: {}, firewall: { enabled: false, protect: false } }
  certificateBaseline.value = ''; loadedConfiguration.value = ''; nodeRevision.value = 0; savedCertificateIds.value = []
  enrollment.value = null; error.value = null; editing.value = true
  if (!node) return
  loading.value = true
  try { const value = await xnodeApi.node(node.id, controller.signal); if (latest.isCurrent(ticket)) { settings.value = value.settings; certificateBaseline.value = JSON.stringify([value.settings.certificates ?? {}, value.settings.inboundCertificates ?? {}]); loadedConfiguration.value = value.node.configId; nodeRevision.value = value.node.desiredRevision ?? 0; savedCertificateIds.value = Object.keys(value.settings.certificates ?? {}); loaded.value = true } }
  catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { if (latest.isCurrent(ticket)) loading.value = false }
}
async function save(): Promise<void> {
  if (!active || loading.value || !loaded.value || blocked.value || !settingsValid.value) return
  const node = cloneForEditing(toRaw(draft))
  const configuration = cloneForEditing(toRaw(settings.value))
  submitting.value = true
  try { if (await props.perform(() => xnodeApi.saveNode(node, configuration)) && active) editing.value = false }
  finally { if (active) submitting.value = false }
}
function retryLoad(): void { const node = props.nodes.find(item => item.id === draft.id); if (node) void edit(node) }
async function enroll(node: XNode): Promise<void> {
  if (!active || blocked.value || enrollingId.value || node.removingAt) return
  const ticket = enrollmentLatest.begin()
  enrollingId.value = node.id
  error.value = null
  try {
    const value = await xnodeApi.enroll(node.id)
    if (enrollmentLatest.isCurrent(ticket)) { enrollment.value = value; enrollmentNode.value = node.name }
  }
  catch (caught) { if (enrollmentLatest.isCurrent(ticket)) error.value = localizedError(caught, 'xnode.saveFailed') }
  finally { if (enrollmentLatest.isCurrent(ticket)) enrollingId.value = '' }
}
</script>

<template>
  <div class="xnode-list">
    <div class="flex justify-end"><UButton icon="i-ph-plus-bold" :disabled="blocked || !configurations.length" :label="t('xnode.addNode')" @click="edit()" /></div>
    <UAlert v-if="!configurations.length" color="neutral" variant="soft" :title="t('xnode.configurationFirst')" />
    <UAlert v-if="error" color="warning" variant="soft" :title="error" />
    <p v-if="!nodes.length" class="text-muted">{{ t('xnode.noNodes') }}</p>
    <div v-for="node in nodes" :key="node.id" class="xnode-row">
      <div><strong>{{ node.name }}</strong><div v-if="node.countryCode" class="text-xs text-muted">{{ node.countryCode }}</div></div>
      <UBadge :color="node.removingAt ? 'warning' : status(node.id)?.connected && status(node.id)?.ready && !status(node.id)?.stale ? 'success' : 'neutral'" variant="subtle">{{ t(node.removingAt ? 'nodeRemoval.pending' : status(node.id)?.connected && status(node.id)?.ready && !status(node.id)?.stale ? 'xnode.online' : 'xnode.offline') }}</UBadge>
      <div class="flex gap-2"><UButton variant="ghost" color="neutral" icon="i-ph-terminal-window" :disabled="blocked || !!node.removingAt" :aria-label="t('nodeAdmin.command')" @click="commanding = node" /><UButton variant="ghost" color="neutral" icon="i-ph-pencil-simple-bold" :disabled="blocked || !!node.removingAt" :aria-label="t('common.edit')" @click="edit(node)" /><UButton variant="ghost" color="neutral" icon="i-ph-key-bold" :loading="enrollingId === node.id" :disabled="!!node.removingAt || !!enrollingId || blocked" :aria-label="t('xnode.enroll')" @click="enroll(node)" /><UButton variant="ghost" color="error" icon="i-ph-trash" :disabled="blocked || !!node.removingAt" :aria-label="t('nodeRemoval.remove')" @click="removing = node" /></div>
    </div>
    <XNodeRemovalDialog :node="removing" @close="removing = null" @removed="emit('refresh')" />
    <UModal :open="editing" :dismissible="!blocked" :close="!blocked" :title="t(draft.id ? 'xnode.editNode' : 'xnode.addNode')" :ui="{ content: 'max-w-4xl' }" @update:open="!blocked && (editing = $event)">
      <template #body>
        <UProgress v-if="loading" />
        <div v-else-if="!loaded" class="grid gap-3"><UAlert color="warning" :title="error ?? t('xnode.loadFailed')" /><UButton class="justify-self-start" variant="outline" color="neutral" :label="t('common.tryAgain')" @click="retryLoad" /></div>
        <template v-else>
          <form class="xnode-form" @submit.prevent="save">
            <fieldset class="xnode-form border-0 p-0 m-0 min-w-0" :disabled="blocked">
              <UAlert v-if="saveError" color="warning" :title="saveError" />
              <div class="xnode-fields">
                <UFormField :label="t('common.name')"><UInput v-model="draft.name" required class="w-full" /></UFormField>
                <UFormField :label="t('xnode.configuration')"><USelect v-model="draft.configId" :items="configurations" class="w-full" /></UFormField>
                <UFormField :label="t('xnode.country')"><UInput v-model="draft.countryCode" maxlength="2" class="w-full" /></UFormField>
                <UFormField :label="t('xnode.provider')"><UInput v-model="draft.providerName" class="w-full" /></UFormField>
                <UFormField><USwitch v-model="draft.enabled" :label="t('common.enabled')" /></UFormField>
              </div>
              <XNodeOperationalSettings v-model="settings" :inbounds="inbounds" :hosts="ddnsHosts" :node-id="draft.id" :revision="nodeRevision" :metric="draft.id ? status(draft.id) : undefined" :certificates-saved="certificatesSaved" :saved-certificate-ids="savedCertificateIds" @valid="settingsValid = $event" />
              <div class="flex justify-end"><UButton type="submit" :loading="blocked" :disabled="!loaded || loading || !settingsValid || blocked" :label="t('common.save')" /></div>
            </fieldset>
          </form>
          <XNodePricingCostSettings v-if="draft.id" :key="draft.id" :node-id="draft.id" />
        </template>
      </template>
    </UModal>
    <UModal :open="!!commanding" :title="t('nodeAdmin.command')" @update:open="!$event && (commanding = null)"><template #body><XNodeCommands v-if="commanding" :key="commanding.id" :node-id="commanding.id" :endpoints="endpoints" /></template></UModal>
    <UModal :open="!!enrollment" :title="t('xnode.enroll')" :description="t('xnode.enrollmentHint')" @update:open="closeEnrollment">
      <template #body><div v-if="enrollment" class="grid gap-4"><p>{{ enrollmentNode }}</p><UFormField :label="t('xnode.masterUrl')"><UInput :model-value="enrollment.masterUrl" readonly class="w-full" /></UFormField><UFormField :label="t('xnode.enrollmentToken')"><UInput :model-value="enrollment.token" readonly class="w-full font-mono" /></UFormField></div></template>
    </UModal>
  </div>
</template>

<style scoped>
.xnode-list, .xnode-form { display: grid; gap: 16px; }
.xnode-row { display: grid; grid-template-columns: minmax(0,1fr) auto auto; gap: 12px; align-items: center; padding-block: 12px; border-bottom: 1px solid var(--line); }
.xnode-fields { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 16px; }
@media (max-width: 600px) { .xnode-fields { grid-template-columns: minmax(0,1fr); } }
</style>
