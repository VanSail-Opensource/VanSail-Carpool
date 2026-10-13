<script setup lang="ts">
import type { USDTQuotaGroup, USDTStatus } from '@/api/usdt'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import { newQuotaGroup } from './configForm'
const model = defineModel<USDTQuotaGroup[]>({ required: true })
const props = defineProps<{ status: USDTStatus | null }>()
const emit = defineEmits<{ rename: [before: string, after: string] }>()
const { t } = useI18n()
const fields = ['requestsPerSecond', 'requestsPerMinute', 'requestsPerDay', 'concurrency'] as const
const maximums = { requestsPerSecond: 50, requestsPerMinute: 3000, requestsPerDay: 1000000, concurrency: 8 }
function update(index: number, field: keyof USDTQuotaGroup, value: string | number): void {
  const current = model.value[index]
  if (!current) return
  const next = { ...current, [field]: value }
  model.value = model.value.map((group, position) => position === index ? next : group)
  if (field === 'id') emit('rename', current.id, String(value))
}
function used(id: string): string {
  const quota = props.status?.quotas.find(item => item.id === id)
  return t('adminUsdt.usage', { minute: quota?.minuteUsed ?? 0, day: quota?.dayUsed ?? 0 })
}
const cooldown = (id: string) => props.status?.quotas.find(item => item.id === id)?.cooldownUntil
</script>

<template>
  <details class="usdt-quotas">
    <summary>{{ t('adminUsdt.quotas') }}</summary>
    <p class="field-hint">{{ t('adminUsdt.quotaHint') }}</p>
    <div v-for="(group, index) in model" :key="index" class="usdt-quota">
      <div class="usdt-toolbar">
        <UFormField :name="`groups.${index}.id`" :label="t('adminUsdt.quotaGroup')"><UInput :model-value="group.id" @update:model-value="update(index, 'id', String($event))" /></UFormField>
        <small>{{ used(group.id) }}</small>
      </div>
      <div class="usdt-quota-grid">
        <UFormField v-for="field in fields" :key="field" :name="`groups.${index}.${field}`" :label="t(`adminUsdt.${field}`)"><UInputNumber :model-value="group[field]" :min="1" :max="maximums[field]" @update:model-value="update(index, field, Number($event))" /></UFormField>
      </div>
      <p v-if="cooldown(group.id)" class="field-hint">{{ t('adminUsdt.cooldown', { time: formatDateTime(cooldown(group.id) ?? undefined) }) }}</p>
    </div>
    <UButton color="neutral" variant="outline" icon="i-ph-plus" :label="t('adminUsdt.addGroup')" :disabled="model.length >= 32" @click="model = [...model, newQuotaGroup()]" />
  </details>
</template>
