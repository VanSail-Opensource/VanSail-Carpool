<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { PaymentReturnProvider } from '@/api/features'
import PaymentReceiptDetails from '@/components/billing/PaymentReceiptDetails.vue'
import { useI18n } from '@/i18n'
import { usePaymentReturn } from '@/composables/usePaymentReturn'
import { isBrowserExperience } from '@/utils/experience'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const orderId = shallowRef('')
const provider = shallowRef<PaymentReturnProvider | null>(null)
const capability = shallowRef('')
const browserReturn = isBrowserExperience()
let homeTimer: ReturnType<typeof globalThis.setTimeout> | undefined

watch(() => [route.query.paymentOrder, route.query.provider, route.query.paymentCapability], ([rawOrder, rawProvider, rawCapability]) => {
  provider.value = rawProvider === 'ezpay' || rawProvider === 'bepusdt' ? rawProvider : null
  capability.value = typeof rawCapability === 'string' ? rawCapability : ''
  orderId.value = typeof rawOrder === 'string' ? rawOrder : ''
}, { immediate: true })

const { state, order, details, orderStatus, isConfirmed } = usePaymentReturn(orderId, {
  browserStatus: browserReturn,
  provider,
  capability,
})
const title = computed(() => ({
  checking: t('payment.returnCheckingTitle'),
  pending: t('payment.returnCheckingTitle'),
  confirmed: t('payment.returnConfirmedTitle'),
  terminal: t('payment.returnTerminalTitle'),
  missing: t('payment.returnMissingTitle'),
  unavailable: t('payment.returnUnavailableTitle'),
})[state.value])
const description = computed(() => state.value === 'terminal'
  ? t('payment.returnTerminalDescription', { status: t(`payment.status.${orderStatus.value ?? details.value?.status ?? 'failed'}`) })
  : t(`payment.return${state.value[0]?.toUpperCase()}${state.value.slice(1)}Description`))
const canReissue = computed(() => order.value?.status === 'expired' || order.value?.status === 'failed')

function reissue(): void {
  if (!canReissue.value || !order.value) return
  void router.replace({ name: 'home', query: { reissue: order.value.id } })
}

watch(isConfirmed, (confirmed) => {
  if (homeTimer !== undefined) globalThis.clearTimeout(homeTimer)
  homeTimer = confirmed && !browserReturn ? globalThis.setTimeout(() => void router.replace('/home'), 2600) : undefined
}, { immediate: true })
onScopeDispose(() => {
  if (homeTimer !== undefined) globalThis.clearTimeout(homeTimer)
})
</script>

<template>
  <PaymentReceiptDetails
    :title="title"
    :description="description"
    :state="state"
    :details="details"
    :can-reissue="canReissue"
    @reissue="reissue"
  />
</template>
