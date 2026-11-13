<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, toRaw, watch } from 'vue'
import { xnodeApi, type NamedService, type XNode, type XNodeGroup } from '@/api/xnode'
import type { XNodeCustomization } from '@/api/xnode'
import JsonDocumentField from '@/components/common/JsonDocumentField.vue'
import { localizedError, useI18n } from '@/i18n'
import { cloneForEditing } from '@/utils/browserCompatibility'
import { createLatestRequest } from '@/utils/latestRequest'

const props = defineProps<{ item: NamedService; nodes: XNode[]; groups: XNodeGroup[] }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const node = shallowRef('')
const current = shallowRef<XNodeCustomization | null>(null)
const outbounds = shallowRef<unknown>([])
const rules = shallowRef<unknown>([])
const error = shallowRef<string | null>(null)
const loading = shallowRef(false)
const busy = shallowRef(false)
const outValid = shallowRef(true)
const rulesValid = shallowRef(true)
const choices = computed(() => {
  const allowed = new Set(props.groups.filter(group => props.item.access.groupIds.includes(group.id)).flatMap(group => group.bindings.map(binding => binding.nodeId)))
  return props.nodes.filter(item => allowed.has(item.id)).map(item => ({ value: item.id, label: item.name }))
})
const reads = createLatestRequest()
const writes = createLatestRequest()
const clone = <T,>(value: T): T => cloneForEditing(toRaw(value))
watch(choices, items => { if (node.value && !items.some(item => item.value === node.value)) node.value = '' }, { flush: 'sync' })
watch([() => props.item.identity.proxyUserId, node], async ([owner, selected]) => {
  const generation = reads.begin()
  writes.invalidate()
  busy.value = false
  loading.value = false
  current.value = null
  outbounds.value = []
  rules.value = []
  error.value = null
  outValid.value = true
  rulesValid.value = true
  if (!selected) return
  loading.value = true
  try {
    const result = await xnodeApi.serviceRouting(owner, selected)
    if (!reads.isCurrent(generation) || owner !== props.item.identity.proxyUserId || selected !== node.value) return
    current.value = result
    outbounds.value = clone(result.outbounds)
    rules.value = clone(result.routing)
  } catch (caught) {
    if (reads.isCurrent(generation)) error.value = localizedError(caught, 'xnode.loadFailed')
  } finally {
    if (reads.isCurrent(generation)) loading.value = false
  }
}, { flush: 'sync' })
async function save(part: 'outbounds' | 'rules') {
  if (!current.value || busy.value || loading.value || (part === 'outbounds' ? !outValid.value : !rulesValid.value)) return
  const owner = props.item.identity.proxyUserId
  const selected = node.value
  const generation = writes.begin()
  const body = {
    ...clone(current.value),
    outbounds: clone(outbounds.value) as XNodeCustomization['outbounds'],
    routing: clone(rules.value) as XNodeCustomization['routing'],
  }
  busy.value = true
  error.value = null
  try {
    const result = await xnodeApi.saveServiceRouting(owner, selected, part, body)
    if (!writes.isCurrent(generation) || owner !== props.item.identity.proxyUserId || selected !== node.value) return
    current.value = result
    // Saving one section must preserve the other section's unsaved draft.
    if (part === 'outbounds') outbounds.value = clone(result.outbounds)
    else rules.value = clone(result.routing)
  } catch (caught) {
    if (writes.isCurrent(generation)) error.value = localizedError(caught, 'xnode.saveFailed')
  } finally {
    if (writes.isCurrent(generation)) busy.value = false
  }
}
onScopeDispose(() => { reads.dispose(); writes.dispose() })
</script>

<template>
  <UModal :open="true" :title="t('memberNodes.routing')" :dismissible="!busy" :close="!busy" @update:open="!$event && !busy && emit('close')">
    <template #body>
      <div class="grid gap-4">
        <UFormField :label="t('xnode.node')"><USelect v-model="node" :items="choices" :disabled="busy" class="w-full" /></UFormField>
        <UAlert v-if="error" color="warning" :title="error" />
        <UProgress v-if="loading" />
        <template v-if="current && !loading">
          <JsonDocumentField v-model="outbounds" :label="t('memberNodes.outbounds')" :disabled="busy" :rows="8" @valid="outValid = $event" />
          <UButton :label="t('memberNodes.saveOutbounds')" :disabled="busy || !outValid" :loading="busy" @click="save('outbounds')" />
          <JsonDocumentField v-model="rules" :label="t('memberNodes.rules')" :disabled="busy" :rows="8" @valid="rulesValid = $event" />
          <UButton :label="t('memberNodes.saveRules')" :disabled="busy || !rulesValid" :loading="busy" @click="save('rules')" />
        </template>
      </div>
    </template>
  </UModal>
</template>
