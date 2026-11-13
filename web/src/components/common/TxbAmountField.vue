<script setup lang="ts">
import { computed } from 'vue'

import { useI18n } from '@/i18n'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'

const props = withDefaults(defineProps<{
  id: string
  label: string
  hint?: string
  minMinor?: string
  maxMinor?: string
  integerOnly?: boolean
  slider?: boolean
  required?: boolean
  disabled?: boolean
  suffix?: string
  displayValue?: (minor: string) => string
  displayMinimum?: (minor: string) => string
  parseDisplayValue?: (value: string) => string
}>(), {
  hint: undefined,
  minMinor: '0',
  maxMinor: undefined,
  integerOnly: false,
  slider: false,
  required: false,
  disabled: false,
  suffix: undefined,
  displayValue: undefined,
  displayMinimum: undefined,
  parseDisplayValue: undefined,
})

const model = defineModel<string>({ required: true })
const { t } = useI18n()
const minor = computed(() => props.parseDisplayValue?.(model.value) ?? moneyFromTxbInput(model.value))
const numericModel = computed<number | null>(() => {
  if (!model.value) return null
  return inputToNumber(model.value) ?? null
})
const minimum = computed(() => inputToNumber(props.displayMinimum?.(props.minMinor) ?? props.displayValue?.(props.minMinor) ?? txbInputFromMinor(props.minMinor)))
const maximum = computed(() => props.maxMinor === undefined ? undefined : inputToNumber(props.displayValue?.(props.maxMinor) ?? txbInputFromMinor(props.maxMinor)))
const showSlider = computed(() => props.slider && minimum.value !== undefined && maximum.value !== undefined && minimum.value < maximum.value)
const sliderValue = computed(() => numericModel.value ?? minimum.value ?? 0)
const sliderStep = computed(() => {
  const range = (maximum.value ?? 0) - (minimum.value ?? 0)
  if (props.integerOnly) return Math.max(1, Math.ceil(range / 200))
  return Math.max(0.01, Math.ceil((range / 200) * 100) / 100)
})
const valid = computed(() => {
  if (minor.value === '') return false
  if (props.integerOnly && !/^\d+$/.test(model.value.trim())) return false
  const value = BigInt(minor.value)
  return value >= BigInt(props.minMinor)
    && (props.maxMinor === undefined || value <= BigInt(props.maxMinor))
})
const message = computed(() => {
  if (!model.value || valid.value) return props.hint ?? t('amount.hint')
  if (minor.value === '') return t('amount.invalidPrecision')
  if (BigInt(minor.value) < BigInt(props.minMinor)) return t('amount.tooSmall')
  return t('amount.tooLarge')
})
const error = computed(() => model.value && !valid.value ? message.value : undefined)

defineExpose({ minor, valid })

function inputToNumber(value: string): number | undefined {
  const parsed = moneyFromTxbInput(value)
  if (!parsed || BigInt(parsed) > BigInt(Number.MAX_SAFE_INTEGER)) return undefined
  const amount = Number(value)
  return Number.isFinite(amount) ? amount : undefined
}

function updateSlider(value: number | number[]): void {
  const next = Array.isArray(value) ? value[0] : value
  if (typeof next === 'number' && Number.isFinite(next)) {
    model.value = props.integerOnly ? String(Math.trunc(next)) : next.toFixed(2)
  }
}
</script>

<template>
  <UFormField
    :name="id"
    :label="label"
    :description="error ? undefined : message"
    :error="error"
    :required="required"
    class="txb-field"
  >
    <div class="amount-input amount-input--compact" :class="{ 'amount-input--slider': showSlider }">
      <UIcon name="i-ph-coins-fill" aria-hidden="true" />
      <UInput
        :id="id"
        v-model="model"
        class="amount-number"
        type="text"
        :inputmode="integerOnly ? 'numeric' : 'decimal'"
        autocomplete="off"
        :required="required"
        :disabled="disabled"
        :aria-invalid="model ? !valid : undefined"
      />
      <span>{{ props.suffix ?? t('common.currencyTxb') }}</span>
      <USlider
        v-if="showSlider"
        class="txb-field__slider"
        :model-value="sliderValue"
        :min="minimum"
        :max="maximum"
        :step="sliderStep"
        :disabled="disabled"
        :aria-label="label"
        @update:model-value="updateSlider"
      />
    </div>
  </UFormField>
</template>

<style scoped>
.txb-field { display: flex; flex-direction: column; gap: 0.4rem; }
.amount-input--compact { width: 100%; min-height: 52px; margin: 0; }
.amount-input--slider { padding-block: 0.6rem; }
.amount-number { width: 100%; min-width: 0; }
.amount-input--compact :deep(input) { min-height: 44px; font-size: 1rem; }
.txb-field__slider { grid-column: 1 / -1; width: 100%; }
</style>
