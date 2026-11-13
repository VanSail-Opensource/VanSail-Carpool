<script setup lang="ts">
import { AnimatePresence, motion } from 'motion-v'

import { useI18n } from '@/i18n'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { selectionHaptic } from '@/utils/telegram'
import type { PaymentProviderOption } from './paymentOptions'

const props = defineProps<{
  options: readonly PaymentProviderOption[]
  selectedValue: string | undefined
  canContinue: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  choose: [value: string]
  continue: []
}>()

const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()

function choose(value: string): void {
  if (props.disabled || value === props.selectedValue) return
  selectionHaptic()
  emit('choose', value)
}
</script>

<template>
  <p v-if="!options.length" role="status" class="field-hint">{{ t('payment.noMethods') }}</p>
  <fieldset v-else class="provider-picker">
    <legend>{{ t('payment.chooseProvider') }}</legend>
    <UButton
      v-for="item in options"
      :key="item.value"
      class="provider-option"
      :class="{ 'provider-option--selected': selectedValue === item.value }"
      color="neutral"
      variant="ghost"
      :disabled="disabled || !item.available"
      :aria-pressed="selectedValue === item.value"
      @click="choose(item.value)"
    >
      <span class="provider-option__icon"><UIcon :name="item.icon" aria-hidden="true" /></span>
      <span><strong>{{ item.label }}</strong><small v-if="item.description">{{ item.description }}</small></span>
      <AnimatePresence :initial="false">
        <motion.span
          v-if="selectedValue === item.value"
          :key="item.value"
          class="provider-option__check"
          :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, scale: 0.84 }"
          :animate="{ opacity: 1, scale: 1 }"
          :exit="{ opacity: 0 }"
          :transition="{ duration: reducedMotion ? 0.08 : 0.14, ease: 'easeOut' }"
        >
          <UIcon name="i-ph-check-circle-fill" aria-hidden="true" />
        </motion.span>
      </AnimatePresence>
    </UButton>
  </fieldset>
  <UButton
    v-if="options.length && selectedValue !== 'coupon'"
    block
    :disabled="disabled || !canContinue"
    :label="t('payment.continueToChannel')"
    data-test="choose-channel"
    data-haptic="navigate"
    @click="emit('continue')"
  />
</template>
