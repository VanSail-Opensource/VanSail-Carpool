<script setup lang="ts">
import { computed } from 'vue'
import type { DDNSHostBinding, XNodeEndpoint, XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'

const props = defineProps<{ job: NonNullable<XNodeSettings['ddns']>[number]; hosts: readonly XNodeEndpoint[]; used: readonly string[]; builtin: boolean }>()
const binding = defineModel<DDNSHostBinding>({ required: true })
const { t } = useI18n()
type HostFamily = 'ipv4HostIds' | 'ipv6HostIds'
type HostField = { key: HostFamily; other: HostFamily; source: string | undefined; label: string }
const fields = computed<HostField[]>(() => [
  { key: 'ipv4HostIds', source: props.job.ipv4, label: t('nodeForms.ddnsHostIPv4'), other: 'ipv6HostIds' },
  { key: 'ipv6HostIds', source: props.job.ipv6, label: t('nodeForms.ddnsHostIPv6'), other: 'ipv4HostIds' },
])
function options(field: HostField) {
  const selected = binding.value[field.key] ?? []
  const other = binding.value[field.other] ?? []
  return props.hosts.filter(host => !other.includes(host.id) && (!props.used.includes(host.id) || selected.includes(host.id)))
    .map(host => ({ value: host.id, label: `${host.name} · ${host.address}` }))
}
function isMultiple(field: HostField): boolean {
  return props.builtin || (binding.value[field.key]?.length ?? 0) > 1
}
function modelValue(field: HostField): string | string[] | undefined {
  const ids = binding.value[field.key] ?? []
  return isMultiple(field) ? ids : ids[0]
}
function select(key: HostFamily, value: unknown): void {
  const ids = Array.isArray(value)
    ? value.filter((id): id is string => typeof id === 'string' && Boolean(id))
    : typeof value === 'string' && value ? [value] : []
  binding.value = { ...binding.value, [key]: ids }
}
</script>

<template>
  <UFormField v-for="field in fields" :key="field.key" :label="field.label" :required="builtin && Boolean(field.source)">
    <USelectMenu :model-value="modelValue(field)" :items="options(field)" value-key="value" :multiple="isMultiple(field)" :disabled="!field.source" :clear="!builtin" :required="builtin && Boolean(field.source)" :placeholder="t(builtin ? 'nodeForms.ddnsHostRequired' : 'nodeForms.ddnsHostNone')" :search-input="{ placeholder: t('nodeForms.discoverySearch'), autofocus: false }" class="w-full" :ui="{ value: 'min-w-0 whitespace-normal break-words text-left', itemLabel: 'whitespace-normal break-words', content: 'max-w-[calc(100vw-2rem)]' }" @update:model-value="select(field.key, $event)">
      <template #empty>{{ t('nodeForms.ddnsHostEmpty') }}</template>
    </USelectMenu>
  </UFormField>
</template>
