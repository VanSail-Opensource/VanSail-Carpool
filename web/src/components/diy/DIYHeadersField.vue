<script setup lang="ts">
import { useI18n } from '@/i18n'
import { useId } from 'vue'
type Header = { name: string; value: string }
const props = defineProps<{ modelValue?: Header[]; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: Header[]] }>()
const { t } = useI18n()
const id = useId()
function update(index: number, key: keyof Header, value: unknown) { emit('update:modelValue', (props.modelValue ?? []).map((row, i) => i === index ? { ...row, [key]: String(value) } : row)) }
</script>
<template>
  <div class="grid gap-2">
    <div v-for="(row, index) in modelValue ?? []" :key="index" class="flex items-start gap-2">
      <div class="grid flex-1 min-w-0 gap-2 sm:grid-cols-2"><UInput :id="`${id}-${index}-name`" :model-value="row.name" :aria-label="t('diy.headerName')" :placeholder="t('diy.headerName')" :disabled="disabled" :maxlength="128" @update:model-value="update(index, 'name', $event)" /><UInput :id="`${id}-${index}-value`" :model-value="row.value" :aria-label="t('diy.headerValue')" :placeholder="t('diy.headerValue')" :disabled="disabled" :maxlength="4096" @update:model-value="update(index, 'value', $event)" /></div>
      <UButton icon="i-ph-trash" color="neutral" variant="ghost" :aria-label="t('common.delete')" :disabled="disabled" @click="emit('update:modelValue', modelValue!.filter((_, i) => i !== index))" />
    </div>
    <UButton class="justify-self-start" icon="i-ph-plus" color="neutral" variant="outline" :label="t('diy.addRow')" :disabled="disabled || (modelValue?.length ?? 0) >= 32" @click="emit('update:modelValue', [...(modelValue ?? []), { name: '', value: '' }])" />
  </div>
</template>
