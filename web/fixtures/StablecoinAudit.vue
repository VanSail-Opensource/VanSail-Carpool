<script setup lang="ts">
import AdminUSDTPage from '@/components/admin/usdt/AdminUSDTPage.vue'
import CryptoPaymentInstructions from '@/components/billing/CryptoPaymentInstructions.vue'
import type { FeaturePaymentOrder } from '@/api/features'
import '@/styles/billing-02.css'
import '@/styles/billing-03.css'

const params = new globalThis.URLSearchParams(globalThis.location.search)
const crypto = params.has('payment')
const stamp = new Date().toISOString()
const order: FeaturePaymentOrder = {
  id: 'fixture-order', methodId: 'builtin_usdt:usdt.erc20', provider: params.has('external') ? 'bepusdt' : 'builtin_usdt',
  providerRail: params.has('usdc') ? 'usdc.erc20' : 'usdt.erc20', status: 'pending', txb: { minor: '1000', display: '10.00', currency: 'TXB' },
  payableAmount: '1.001', payableCurrency: 'USDT', rateSnapshot: '10', rateDirection: 'txb_per_currency',
  paymentUrl: null, qrPayload: null, receivingAddress: '0x1111111111111111111111111111111111111111',
  actualCryptoAmount: '1.001', actualCryptoCurrency: params.has('usdc') ? 'USDC' : 'USDT',
  expiresAt: new Date(Date.now() + (params.has('expired') ? -1000 : 1800000)).toISOString(),
  paidAt: null, refundedAt: null, cancelledAt: null, cancelReason: '', providerCancelStatus: '', createdAt: stamp, updatedAt: stamp,
}
</script>

<template><UApp><main class="stablecoin-audit"><CryptoPaymentInstructions v-if="crypto" :order="order" stage="pending" :qr-data-url="null" :error="null" :can-recreate="true" /><AdminUSDTPage v-else /></main></UApp></template>

<style scoped>
.stablecoin-audit { max-width: 1100px; margin-inline: auto; padding: 20px; }
</style>
