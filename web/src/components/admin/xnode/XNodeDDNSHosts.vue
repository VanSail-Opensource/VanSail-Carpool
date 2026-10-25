<script setup lang="ts">
import { computed } from 'vue'
import type { DDNSHostBinding, XNodeEndpoint, XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'

const props = defineProps<{ job: NonNullable<XNodeSettings['ddns']>[number]; hosts: readonly XNodeEndpoint[]; used: readonly string[]; builtin: boolean }>()
const binding = defineModel<DDNSHostBinding>({ required: true })
const { t } = useI18n()
const fields = computed(() => [
  { key: 'ipv4HostId' as const, source: props.job.ipv4, label: t('nodeForms.ddnsHostIPv4'), other: binding.value.ipv6HostId },
  { key: 'ipv6HostId' as const, source: props.job.ipv6, label: t('nodeForms.ddnsHostIPv6'), other: binding.value.ipv4HostId },
])
function options(field: typeof fields.value[number]) {
  return props.hosts.filter(host => host.id !== field.other && (!props.used.includes(host.id) || host.id === binding.value[field.key]))
    .map(host => ({ value: host.id, label: `${host.name} · ${host.address}` }))
}
function select(key: 'ipv4HostId' | 'ipv6HostId', value: unknown): void {
  binding.value = { ...binding.value, [key]: typeof value === 'string' && value ? value : undefined }
}
</script>

<template>
  <UFormField v-for="field in fields" :key="field.key" :label="field.label" :required="builtin && Boolean(field.source)">
    <USelectMenu :model-value="binding[field.key]" :items="options(field)" value-key="value" :disabled="!field.source" :clear="!builtin" :required="builtin && Boolean(field.source)" :placeholder="t(builtin ? 'nodeForms.ddnsHostRequired' : 'nodeForms.ddnsHostNone')" :search-input="{ placeholder: t('nodeForms.discoverySearch'), autofocus: false }" class="w-full" :ui="{ value: 'min-w-0 whitespace-normal break-words text-left', itemLabel: 'whitespace-normal break-words', content: 'max-w-[calc(100vw-2rem)]' }" @update:model-value="select(field.key, $event)">
      <template #empty>{{ t('nodeForms.ddnsHostEmpty') }}</template>
    </USelectMenu>
  </UFormField>
</template>
