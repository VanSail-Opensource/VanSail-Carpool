<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import { useI18n } from '@/i18n'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { selectionHaptic } from '@/utils/telegram'
import { paymentCurrencyLogo, paymentNetworkLogo, type PaymentChannelOption } from './paymentOptions'

const props = defineProps<{
  channels: readonly PaymentChannelOption[]
  selectedMethodId: string | null
}>()
const emit = defineEmits<{ choose: [id: string] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()

const currencies = computed(() => {
  const seen = new Set<string>()
  return props.channels.filter((channel) => {
    if (!channel.cryptoCurrency || seen.has(channel.cryptoCurrency)) return false
    seen.add(channel.cryptoCurrency)
    return true
  }).map(channel => ({
    ...channel,
    disabled: !props.channels.some(candidate => candidate.cryptoCurrency === channel.cryptoCurrency && !candidate.disabled),
  }))
})
const selectedCurrency = computed(() => props.channels.find((item) => item.value === props.selectedMethodId)?.cryptoCurrency
  ?? currencies.value[0]?.cryptoCurrency)
const networks = computed(() => props.channels.filter((channel) => channel.cryptoCurrency === selectedCurrency.value))

function choose(id: string): void {
  if (id === props.selectedMethodId) return
  selectionHaptic()
  emit('choose', id)
}

function chooseCurrency(currency: 'USDT' | 'USDC'): void {
  const channel = props.channels.find((item) => item.cryptoCurrency === currency && !item.disabled)
  if (channel) choose(channel.value)
}
</script>

<template>
  <fieldset class="crypto-picker">
    <legend>{{ t('cryptoPayment.chooseCurrency') }}</legend>
    <div class="crypto-picker__currencies">
      <UButton
        v-for="item in currencies"
        :key="item.cryptoCurrency"
        class="crypto-currency"
        :class="{ 'crypto-currency--selected': selectedCurrency === item.cryptoCurrency }"
        color="neutral"
        variant="ghost"
        :disabled="item.disabled"
        :aria-pressed="selectedCurrency === item.cryptoCurrency"
        @click="item.cryptoCurrency && chooseCurrency(item.cryptoCurrency)"
      >
        <img :src="paymentCurrencyLogo(item.cryptoCurrency!)" alt="" width="34" height="34" />
        <strong>{{ item.cryptoCurrency }}</strong>
        <AnimatePresence :initial="false">
          <motion.span
            v-if="selectedCurrency === item.cryptoCurrency"
            :key="item.cryptoCurrency"
            :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, scale: 0.84 }"
            :animate="{ opacity: 1, scale: 1 }"
            :exit="{ opacity: 0 }"
            :transition="{ duration: reducedMotion ? 0.08 : 0.14, ease: 'easeOut' }"
          >
            <UIcon name="i-ph-check-circle-fill" aria-hidden="true" />
          </motion.span>
        </AnimatePresence>
      </UButton>
    </div>
  </fieldset>

  <fieldset class="crypto-picker">
    <legend>{{ t('cryptoPayment.chooseNetwork') }}</legend>
    <div class="crypto-picker__networks">
      <UButton
        v-for="item in networks"
        :key="item.value"
        class="provider-option crypto-network"
        :class="{ 'provider-option--selected': selectedMethodId === item.value }"
        color="neutral"
        variant="ghost"
        :disabled="item.disabled"
        :aria-pressed="selectedMethodId === item.value"
        @click="choose(item.value)"
      >
        <span class="provider-option__icon">
          <img :src="paymentNetworkLogo(item.network ?? '', item.cryptoCurrency!)" alt="" width="28" height="28" />
        </span>
        <span><strong>{{ item.networkName || item.label }}</strong></span>
        <AnimatePresence :initial="false">
          <motion.span
            v-if="selectedMethodId === item.value"
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
    </div>
  </fieldset>
</template>
