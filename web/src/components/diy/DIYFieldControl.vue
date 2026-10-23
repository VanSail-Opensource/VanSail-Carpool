<script setup lang="ts">
import { computed, useId } from 'vue'
import type { DIYField, DIYOverrides } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import { selectionHaptic } from '@/utils/telegramHaptics'
import DIYRangeField from './DIYRangeField.vue'
import DIYHeadersField from './DIYHeadersField.vue'
const props = defineProps<{ field: DIYField; modelValue: unknown; disabled?: boolean; error?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: unknown] }>()
const { t } = useI18n()
const id = useId()
const inherit = '_inherit'
const fixedOff = computed(() => props.field.key === 'tcpMptcp')
const enumItems = computed(() => [{ value: inherit, label: t('diy.inherit') }, ...(props.field.enum ?? []).map(value => ({ value, label: value || t('diy.system') }))])
const booleanItems = computed(() => [{ value: inherit, label: t('diy.inherit') }, { value: 'true', label: t('diy.enabled'), disabled: props.field.restrictions?.includes('serverCapability') }, { value: 'false', label: t('diy.disabled') }])
const fastItems = computed(() => [...booleanItems.value, { value: '0', label: t('diy.system') }, { value: '-1', label: '-1' }, { value: '_number', label: t('diy.custom') }])
const scalar = computed(() => typeof props.modelValue === 'number' || typeof props.modelValue === 'string' ? props.modelValue : undefined)
const headers = computed(() => props.modelValue as DIYOverrides['xhttpHeaders'])
const strings = computed(() => Array.isArray(props.modelValue) ? props.modelValue as string[] : [])
const selected = computed(() => props.modelValue === undefined ? inherit : String(props.modelValue))
const fastSelected = computed(() => typeof props.modelValue === 'number' && ![0, -1].includes(props.modelValue) ? '_number' : selected.value)
function choose(value: unknown) {
  selectionHaptic()
  if (value === inherit) emit('update:modelValue', undefined)
  else if (props.field.kind === 'boolean' || props.field.kind === 'fast-open' && ['true', 'false'].includes(String(value))) emit('update:modelValue', value === 'true')
  else if (props.field.kind === 'enum-number' || props.field.kind === 'fast-open') emit('update:modelValue', value === '_number' ? 1 : Number(value))
  else emit('update:modelValue', value)
}
function number(value: unknown) { emit('update:modelValue', value === '' || value === undefined ? undefined : Number.isFinite(Number(value)) ? Number(value) : value) }
</script>

<template>
  <UFormField :label="t(`diyFields.${field.key}`)" :hint="fixedOff ? t('diy.states.fixed-off') : undefined" :error="error || (field.kind === 'range' && typeof modelValue === 'string' ? t('diy.rangeInvalid') : undefined)" :description="field.unit ? t(`diy.units.${field.unit}`) : undefined" class="min-w-0">
    <div class="flex items-start gap-1">
      <div class="grid flex-1 min-w-0 gap-2">
        <UInput v-if="fixedOff" :model-value="t('diy.disabled')" disabled class="w-full" />
        <USelect v-else-if="field.kind === 'boolean'" :model-value="selected" :items="booleanItems" :disabled="disabled" class="w-full" @update:model-value="choose" />
        <USelect v-else-if="field.kind === 'enum' || field.kind === 'enum-number'" :model-value="selected" :items="enumItems" :disabled="disabled" :ui="{ value: 'whitespace-normal break-words', itemLabel: 'whitespace-normal break-words' }" class="w-full" @update:model-value="choose" />
        <template v-else-if="field.kind === 'fast-open'"><USelect :model-value="fastSelected" :items="fastItems" :disabled="disabled" class="w-full" @update:model-value="choose" /><UInput :id="`${id}-count`" v-if="fastSelected === '_number'" :model-value="scalar" type="number" :min="field.minimum" :max="field.maximum" step="1" :disabled="disabled" :aria-label="t(`diyFields.${field.key}`)" class="w-full" @update:model-value="number" /></template>
        <DIYRangeField v-else-if="field.kind === 'range'" :model-value="modelValue" :label="t(`diyFields.${field.key}`)" :disabled="disabled" @update:model-value="emit('update:modelValue', $event)" />
        <DIYHeadersField v-else-if="field.kind === 'headers'" :model-value="headers" :disabled="disabled" @update:model-value="emit('update:modelValue', $event)" />
        <USelectMenu v-else-if="field.kind === 'strings'" :model-value="strings" multiple :placeholder="t(modelValue === undefined ? 'diy.inherit' : 'diy.system')" :items="field.enum ?? []" :disabled="disabled" :search-input="{ placeholder: t('diy.custom') }" class="w-full" @update:model-value="selectionHaptic(); emit('update:modelValue', $event)" />
        <UInput v-else-if="field.kind === 'integer'" :model-value="scalar" type="number" :min="field.minimum" :max="field.maximum" step="1" :disabled="disabled" :placeholder="t('diy.inherit')" class="w-full" @update:model-value="number" />
        <UInput v-else :model-value="typeof modelValue === 'string' ? modelValue : undefined" :maxlength="field.maximum" :disabled="disabled" :placeholder="t('diy.inherit')" class="w-full" @update:model-value="emit('update:modelValue', $event)" />
      </div>
    </div>
  </UFormField>
</template>
