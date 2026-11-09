<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { objectOption } from './nodeFormOptions'
const props = defineProps<{ modelValue?: unknown; disabled?: boolean; label: string; error?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: unknown] }>()
const { t } = useI18n()
const rows = computed(() => Array.isArray(props.modelValue) ? props.modelValue.map(objectOption) : [])
const mode = computed({ get: () => props.modelValue === undefined ? 'inherit' : rows.value.length ? 'on' : 'off', set: value => emit('update:modelValue', value === 'inherit' ? undefined : value === 'off' ? [] : [{ min: 443, max: 443 }]) })
const modes = computed(() => [{ value: 'inherit', label: t('diy.inherit') }, { value: 'on', label: t('diy.enabled') }, { value: 'off', label: t('diy.disabled') }])
function change(index: number, key: 'min' | 'max', value: unknown) {
  const next = rows.value.map(row => ({ ...row }))
  next[index] = { ...next[index], [key]: value === '' ? '' : Number(value) }
  emit('update:modelValue', next)
}
function remove(index: number) { emit('update:modelValue', rows.value.filter((_, i) => i !== index)) }
</script>
<template>
  <UFormField :label="label" :error="error" :description="t('endpointOptions.portsHint')" class="sm:col-span-2 min-w-0">
    <div class="grid gap-2">
      <USelect v-model="mode" :items="modes" :disabled="disabled" class="w-full" />
      <div v-for="(row, index) in rows" :key="index" class="flex items-start gap-2">
        <div class="grid min-w-0 flex-1 grid-cols-2 gap-2">
          <UInput :model-value="typeof row.min === 'number' ? row.min : ''" type="number" min="1" max="65535" step="1" :aria-label="t('endpointOptions.portFrom')" :disabled="disabled" class="w-full" @update:model-value="change(index, 'min', $event)" />
          <UInput :model-value="typeof row.max === 'number' ? row.max : ''" type="number" min="1" max="65535" step="1" :aria-label="t('endpointOptions.portTo')" :disabled="disabled" class="w-full" @update:model-value="change(index, 'max', $event)" />
        </div>
        <UButton icon="i-ph-trash" color="neutral" variant="ghost" :aria-label="t('common.delete')" :disabled="disabled" @click="remove(index)" />
      </div>
      <UButton v-if="mode === 'on'" icon="i-ph-plus" color="neutral" variant="outline" :label="t('diy.addRow')" :disabled="disabled || rows.length >= 64" class="justify-self-start" @click="emit('update:modelValue', [...rows, { min: 443, max: 443 }])" />
    </div>
  </UFormField>
</template>
