<script setup lang="ts">
import { useI18n } from '@/i18n'
import { useId } from 'vue'
defineProps<{ modelValue?: { min: number; max: number }; disabled?: boolean; label: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: { min: number; max: number } | undefined] }>()
const { t } = useI18n()
const id = useId()
function change(side: 'min' | 'max', value: string | number | undefined, current?: { min: number; max: number }) {
  if (value === '' || value === undefined) { emit('update:modelValue', undefined); return }
  const number = Number(value)
  emit('update:modelValue', { ...(current ?? { min: number, max: number }), [side]: number })
}
</script>
<template>
  <div role="group" :aria-label="label" class="grid grid-cols-2 gap-2">
    <UInput :id="`${id}-min`" :model-value="modelValue?.min" type="number" min="0" max="2147483647" step="1" :aria-label="`${label}: ${t('diy.minimum')}`" :placeholder="t('diy.inherit')" :disabled="disabled" class="w-full" @update:model-value="change('min', $event, modelValue)" />
    <UInput :id="`${id}-max`" :model-value="modelValue?.max" type="number" min="0" max="2147483647" step="1" :aria-label="`${label}: ${t('diy.maximum')}`" :placeholder="t('diy.inherit')" :disabled="disabled" class="w-full" @update:model-value="change('max', $event, modelValue)" />
  </div>
</template>
