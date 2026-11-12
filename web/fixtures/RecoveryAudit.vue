<script setup lang="ts">
import { shallowRef } from 'vue'
import type { FeaturePaymentMethod, FeaturePaymentOrder } from '@/api/features'
import CurrencyControl from '@/components/layout/CurrencyControl.vue'
import BalancePaymentSheet from '@/components/billing/BalancePaymentSheet.vue'
import { useDisplayCurrency } from '@/composables/useDisplayCurrency'
import { useSessionStore } from '@/stores/session'

defineProps<{ methods: FeaturePaymentMethod[]; pendingOrder: FeaturePaymentOrder | null }>()
const currency = useDisplayCurrency()
const session = useSessionStore()
const open = shallowRef(false)
</script>

<template>
  <main class="recovery-audit">
    <h1>{{ $t('settings.title') }}</h1>
    <CurrencyControl show-label />
    <output data-test="currency-value">{{ currency.currency.value }}</output>
    <output data-test="currency-loading">{{ currency.loading.value }}</output>
    <div class="recovery-audit__actions">
      <UButton data-test="refresh-currency" :label="$t('common.refresh')" @click="currency.refresh()" />
      <UButton data-test="open-checkout" :label="$t('payment.addTxb')" @click="open = true" />
      <UButton data-test="clear-session" :label="$t('browser.logout')" @click="session.clear()" />
    </div>
    <BalancePaymentSheet v-model:open="open" :methods="methods" :pending-order="pendingOrder" />
  </main>
</template>

<style scoped>
.recovery-audit { display: grid; gap: 1rem; width: min(100%, 40rem); margin: auto; padding: 1.5rem; }
.recovery-audit__actions { display: flex; flex-wrap: wrap; gap: 0.5rem; }
</style>
