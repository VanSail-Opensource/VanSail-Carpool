<script setup lang="ts">
import { useI18n } from '@/i18n'
import { formatDIYRange, parseDIYRange } from './diyRange'

defineProps<{ modelValue?: unknown; disabled?: boolean; label: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: unknown] }>()
const { t } = useI18n()
function change(value: string | number) {
  const text = String(value)
  emit('update:modelValue', text.trim() ? parseDIYRange(text) ?? text : undefined)
}
</script>
<template>
  <UInput :model-value="formatDIYRange(modelValue)" :aria-label="label" :aria-invalid="typeof modelValue === 'string'" :placeholder="t('diy.rangeExample')" :disabled="disabled" inputmode="text" class="w-full" @update:model-value="change" />
</template>
