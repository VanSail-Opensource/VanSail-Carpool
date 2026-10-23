<script setup lang="ts">
import type { TableColumn } from '@nuxt/ui'
import { computed } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'
type Policy = NonNullable<XNodeSettings['firewall']>
const model = defineModel<Policy>({ required: true })
defineProps<{ rulesOnly?: boolean }>()
const { t } = useI18n()
const names = ['closed_port', 'invalid_packet', 'syn_flood', 'icmp_probe', 'xray_tcp_probe', 'xray_invalid', 'xray_auth_failure']
const rules = computed(() => names.map(name => ({ name, value: model.value.rules?.[name] ?? { enabled: true, threshold: 3, window_seconds: 86400 } })))
function update(name: string, part: 'enabled' | 'threshold' | 'window_seconds', value: unknown): void {
  const previous = model.value.rules?.[name] ?? { enabled: true, threshold: 3, window_seconds: 86400 }
  model.value = { ...model.value, rules: { ...model.value.rules, [name]: { ...previous, [part]: value } } }
}
const columns = computed<TableColumn<{ name: string; value: { enabled: boolean; threshold: number; window_seconds: number } }>[]>(() => [
  { id: 'rule', header: t('firewall.rule') }, { id: 'threshold', header: t('firewall.threshold') }, { id: 'window', header: t('firewall.windowSeconds') },
])
const seconds = computed({ get: () => model.value.ban_seconds || 259200, set: value => { model.value = { ...model.value, ban_seconds: value } } })
</script>

<template>
  <div class="grid gap-4">
    <USwitch v-if="!rulesOnly" :model-value="model.enabled" :label="t('common.enabled')" @update:model-value="model = { ...model, enabled: $event }" />
    <USwitch v-if="!rulesOnly" :model-value="model.protect" :label="t('xnode.probeProtection')" :description="t('firewall.detectionHint')" @update:model-value="model = { ...model, protect: $event }" />
    <UFormField v-if="rulesOnly" :label="t('firewall.banSeconds')"><UInput v-model.number="seconds" type="number" min="1" max="31536000" required /></UFormField>
    <div v-if="rulesOnly" class="overflow-x-auto">
      <UTable :data="rules" :columns="columns" class="admin-table">
        <template #rule-cell="{ row }"><USwitch :model-value="row.original.value.enabled" :label="t(`firewall.rules.${row.original.name}`)" @update:model-value="update(row.original.name, 'enabled', $event)" /></template>
        <template #threshold-cell="{ row }"><UInput :model-value="row.original.value.threshold" type="number" min="1" max="1000000" required class="w-24" :aria-label="t('firewall.threshold')" @update:model-value="update(row.original.name, 'threshold', Number($event))" /></template>
        <template #window-cell="{ row }"><UInput :model-value="row.original.value.window_seconds" type="number" min="1" max="31536000" required class="w-32" :aria-label="t('firewall.windowSeconds')" @update:model-value="update(row.original.name, 'window_seconds', Number($event))" /></template>
      </UTable>
    </div>
  </div>
</template>
