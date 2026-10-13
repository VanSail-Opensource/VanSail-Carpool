<script setup lang="ts">
import { computed } from 'vue'
import type { USDTEndpoint, USDTNetworkConfig, USDTQuotaGroup } from '@/api/usdt'
import SwitchField from '@/components/common/SwitchField.vue'
import { useI18n } from '@/i18n'
import { newEndpoint } from './configForm'
import USDTEndpointEditor from './USDTEndpointEditor.vue'
const model = defineModel<USDTNetworkConfig>({ required: true })
const props = defineProps<{ groups: USDTQuotaGroup[]; index: number }>()
const emit = defineEmits<{ ensureGroup: [] }>()
const { t } = useI18n()
const addresses = computed({ get: () => model.value.addresses.join('\n'), set: (value: string) => { model.value = { ...model.value, addresses: value.split(/\r?\n/).map(line => line.trim()).filter(Boolean) } } })
function addEndpoint(): void {
  if (!props.groups.length) emit('ensureGroup')
  model.value = { ...model.value, endpoints: [...model.value.endpoints, newEndpoint(props.groups[0]?.id ?? 'rpc-default')] }
}
function move(index: number, direction: number): void {
  const endpoints = [...model.value.endpoints]
  const next = index + direction
  if (next < 0 || next >= endpoints.length) return
  ;[endpoints[index], endpoints[next]] = [endpoints[next]!, endpoints[index]!]
  model.value = { ...model.value, endpoints }
}
function remove(index: number): void { model.value = { ...model.value, endpoints: model.value.endpoints.filter((_, position) => position !== index) } }
function updateEndpoint(index: number, endpoint: USDTEndpoint): void { const endpoints = [...model.value.endpoints]; endpoints[index] = endpoint; model.value = { ...model.value, endpoints } }
</script>

<template>
  <section class="usdt-network">
    <SwitchField :id="`usdt-network-${model.network}`" :model-value="model.enabled" :label="t('adminUsdt.enableNetwork')" @update:model-value="model = { ...model, enabled: $event }" />
    <UFormField :name="`networks.${index}.addresses`" :label="t('adminUsdt.addresses')" :description="t('adminUsdt.addressesHint')"><UTextarea v-model="addresses" class="usdt-addresses" :rows="3" autoresize :maxrows="8" spellcheck="false" /></UFormField>
    <UAlert v-if="!model.endpoints.length" color="neutral" variant="soft" :description="t('adminUsdt.noEndpoints')" />
    <USDTEndpointEditor v-for="(endpoint, position) in model.endpoints" :key="endpoint.id" :model-value="endpoint" :network="model.network" :groups="groups" :index="position" :last="position === model.endpoints.length - 1" :prefix="`networks.${index}.endpoints.${position}`" @update:model-value="updateEndpoint(position, $event)" @remove="remove(position)" @move="move(position, $event)" />
    <UButton color="neutral" variant="outline" icon="i-ph-plus" :label="t('adminUsdt.addEndpoint')" :disabled="model.endpoints.length >= 8" @click="addEndpoint" />
  </section>
</template>
