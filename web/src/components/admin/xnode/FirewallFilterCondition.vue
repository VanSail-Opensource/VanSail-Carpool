<script setup lang="ts">
import { computed } from 'vue'
import type { FirewallFilter } from '@/api/firewall'
import { useI18n } from '@/i18n'

const props = defineProps<{ modelValue: FirewallFilter }>()
const emit = defineEmits<{ 'update:modelValue': [value: FirewallFilter] }>()
const { t } = useI18n()
const fields = computed(() => ['ip', 'rule', 'status', 'hits', 'detectedAt'].map(value => ({ value, label: t(`firewall.filterFields.${value}`) })))
const rules = computed(() => ['closed_port', 'invalid_packet', 'syn_flood', 'icmp_probe', 'xray_tcp_probe', 'xray_invalid', 'xray_auth_failure', 'source_match'].map(value => ({ value, label: t(`firewall.rules.${value}`) })))
const statuses = computed(() => [{ value: 'banned', label: t('firewall.banned') }, { value: 'reported', label: t('firewall.reported') }])

function setField(value: unknown) {
  const field = String(value) as FirewallFilter['field']
  const defaults: Record<string, string> = { rule: 'xray_tcp_probe', status: 'reported', hits: '1', ip: '' }
  emit('update:modelValue', field === 'detectedAt' ? { field } : { field, value: defaults[field ?? ''] ?? '' })
}
function setValue(value: unknown) { emit('update:modelValue', { ...props.modelValue, value: String(value ?? '') }) }
function localTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16)
}
function setTime(key: 'from' | 'to', value: unknown) {
  const raw = String(value ?? '')
  const date = new Date(raw)
  emit('update:modelValue', { ...props.modelValue, [key]: raw && !Number.isNaN(date.getTime()) ? date.toISOString() : undefined })
}
</script>

<template>
  <div class="flex min-w-0 flex-1 flex-wrap items-center gap-2">
    <USelect :model-value="modelValue.field" :items="fields" :aria-label="t('firewall.filterField')" class="w-40 max-w-full" @update:model-value="setField" />
    <USelect v-if="modelValue.field === 'rule'" :model-value="modelValue.value" :items="rules" :aria-label="t('firewall.rule')" class="min-w-40 flex-1" @update:model-value="setValue" />
    <USelect v-else-if="modelValue.field === 'status'" :model-value="modelValue.value" :items="statuses" :aria-label="t('firewall.status')" class="min-w-36 flex-1" @update:model-value="setValue" />
    <template v-else-if="modelValue.field === 'detectedAt'">
      <UInput type="datetime-local" :model-value="localTime(modelValue.from)" :aria-label="t('firewall.from')" class="min-w-40 flex-1" @update:model-value="setTime('from', $event)" />
      <UInput type="datetime-local" :model-value="localTime(modelValue.to)" :aria-label="t('firewall.to')" class="min-w-40 flex-1" @update:model-value="setTime('to', $event)" />
    </template>
    <UInput v-else :model-value="modelValue.value" :type="modelValue.field === 'hits' ? 'number' : 'text'" :min="modelValue.field === 'hits' ? 0 : undefined" :placeholder="modelValue.field === 'ip' ? t('firewall.searchHint') : undefined" :aria-label="t(modelValue.field === 'hits' ? 'firewall.minHits' : 'firewall.ip')" class="min-w-32 flex-1" @update:model-value="setValue" />
  </div>
</template>
