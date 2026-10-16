<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from '@/i18n'

const props = defineProps<{ modelValue: unknown; label: string; disabled?: boolean; rows?: number }>()
const emit = defineEmits<{ 'update:modelValue': [value: unknown]; valid: [valid: boolean] }>()
const { t } = useI18n()
const text = shallowRef(JSON.stringify(props.modelValue, null, 2))
const invalid = computed(() => { try { JSON.parse(text.value); return false } catch { return true } })
watch(() => props.modelValue, value => {
  text.value = JSON.stringify(value, null, 2)
  emit('valid', true)
}, { immediate: true })
function update(value: string): void {
  text.value = value
  emit('valid', !invalid.value)
  if (!invalid.value) emit('update:modelValue', JSON.parse(value))
}
</script>

<template>
  <UFormField :label="label" :error="invalid ? t('xnode.invalidJson') : undefined" class="w-full">
    <UTextarea :model-value="text" :rows="rows ?? 14" :disabled="disabled" :aria-label="label" spellcheck="false" class="w-full font-mono" @update:model-value="update" />
  </UFormField>
</template>
