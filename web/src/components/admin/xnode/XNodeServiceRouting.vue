<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { xnodeApi, type NamedService, type XNode, type XNodeGroup } from '@/api/xnode'
import type { XNodeCustomization } from '@/api/xnodeMember'
import JsonDocumentField from '@/components/common/JsonDocumentField.vue'
import { localizedError, useI18n } from '@/i18n'

const props = defineProps<{ item: NamedService; nodes: XNode[]; groups: XNodeGroup[] }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const node = shallowRef('')
const current = ref<XNodeCustomization | null>(null)
const outbounds = ref<unknown>([])
const rules = ref<unknown>([])
const error = shallowRef<string | null>(null)
const loading = shallowRef(false)
const busy = shallowRef(false)
const outValid = shallowRef(true)
const rulesValid = shallowRef(true)
const choices = computed(() => {
  const allowed = new Set(props.groups.filter(group => props.item.access.groupIds.includes(group.id)).flatMap(group => group.bindings.map(binding => binding.nodeId)))
  return props.nodes.filter(item => allowed.has(item.id)).map(item => ({ value: item.id, label: item.name }))
})
watch(node, async (value, _, cleanup) => {
  let active = true
  cleanup(() => { active = false })
  current.value = null; error.value = null
  if (!value) return
  loading.value = true
  try {
    const result = await xnodeApi.serviceRouting(props.item.identity.proxyUserId, value)
    if (active) { current.value = result; outbounds.value = result.outbounds; rules.value = result.routing }
  } catch (caught) { if (active) error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { if (active) loading.value = false }
})
async function save(part: 'outbounds' | 'rules') {
  if (!current.value || busy.value) return
  busy.value = true; error.value = null
  try {
    current.value = await xnodeApi.saveServiceRouting(props.item.identity.proxyUserId, node.value, part, { ...current.value, outbounds: outbounds.value as XNodeCustomization['outbounds'], routing: rules.value as XNodeCustomization['routing'] })
    outbounds.value = current.value.outbounds; rules.value = current.value.routing
  } catch (caught) { error.value = localizedError(caught, 'xnode.saveFailed') }
  finally { busy.value = false }
}
</script>

<template>
  <UModal :open="true" :title="t('memberNodes.routing')" @update:open="!$event && emit('close')">
    <template #body>
      <div class="grid gap-4">
        <UFormField :label="t('xnode.node')"><USelect v-model="node" :items="choices" :disabled="busy" class="w-full" /></UFormField>
        <UAlert v-if="error" color="warning" :title="error" />
        <UProgress v-if="loading" />
        <template v-if="current && !loading">
          <JsonDocumentField v-model="outbounds" :label="t('memberNodes.outbounds')" :disabled="busy" :rows="8" @valid="outValid = $event" />
          <UButton :label="t('memberNodes.saveOutbounds')" :disabled="!outValid" :loading="busy" @click="save('outbounds')" />
          <JsonDocumentField v-model="rules" :label="t('memberNodes.rules')" :disabled="busy" :rows="8" @valid="rulesValid = $event" />
          <UButton :label="t('memberNodes.saveRules')" :disabled="!rulesValid" :loading="busy" @click="save('rules')" />
        </template>
      </div>
    </template>
  </UModal>
</template>
