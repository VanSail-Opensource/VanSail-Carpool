<script setup lang="ts">
import { shallowRef } from 'vue'
import { api } from '@/api/client'
import type { Purchase } from '@/api/types'
import EntitlementSummary from '@/components/dashboard/EntitlementSummary.vue'
import AutoRenewalControl from '@/components/dashboard/AutoRenewalControl.vue'
import { installRenewalPricingFixture } from './renewalPricingFixture'

const params = new globalThis.URLSearchParams(globalThis.location.search)
const now = Date.now()
const iso = (offset: number) => new Date(now + offset).toISOString()
const money = { currency: 'TXB' as const, minor: '1000', display: '10.00 TXB' }
const initial: Purchase = { id: 'queued-1', comboId: 'combo-1', comboName: 'Constructed next cycle', status: 'queued', price: money, grossPrice: money, couponDiscount: { ...money, minor: '0', display: '0.00 TXB' }, couponGrantId: null, trafficLimitBytes: '1000000000', resetStrategy: 'MONTH_ROLLING', squadUuids: [], autoRenewEnabled: false, validFrom: iso(86_400_000), validUntil: iso(172_800_000), createdAt: iso(0), updatedAt: iso(0) }
const queued = shallowRef<Purchase | null>(initial)
const renewalPurchase: Purchase = { ...initial, id: 'renewal-purchase', status: 'active', validFrom: iso(-86400_000) }
if (params.has('renewal')) installRenewalPricingFixture(renewalPurchase, params)
const state = { cancels: [] as string[], changed: 0, fail: params.has('error'), finished: 0 }
let release: (() => void) | undefined
api.cancelQueuedPurchase = async id => {
  const purchase = queued.value ?? initial
  state.cancels.push(id)
  if (params.has('defer')) await new Promise<void>(done => { release = done })
  state.finished++
  if (state.fail) throw new Error('Constructed cancellation refusal')
  return { ...purchase, id, status: 'cancelled' }
}
Object.assign(globalThis, { __purchaseRecoveryAudit: {
  state, replace: () => { queued.value = { ...initial, id: 'queued-2', comboName: 'Replacement next cycle' } },
  release: () => release?.(),
} })
function cancelled(): void { state.changed++; queued.value = null }
</script>
<template>
  <UApp><main class="purchase-recovery-audit"><h1>{{ $t('hourly.legacyLabel') }}</h1><AutoRenewalControl v-if="params.has('renewal')" :purchase="renewalPurchase" /><EntitlementSummary v-else :queued="queued" @queued-cancelled="cancelled" /></main></UApp>
</template>
<style scoped>.purchase-recovery-audit { width: min(100%, 50rem); margin: auto; padding: 1rem; }</style>
