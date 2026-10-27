<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { PricingProduct } from '@/api/pricingAnalysis'
import { useI18n } from '@/i18n'
import { pricingLabel, pricingMoney } from '@/utils/pricingDisplay'
const props = defineProps<{ products: PricingProduct[] }>()
const { t } = useI18n()
const now = shallowRef(Date.now())
watch(() => props.products, (products, _, cleanup) => {
  let timer: ReturnType<typeof globalThis.setTimeout> | undefined
  function schedule() {
    now.value = Date.now()
    const next = Math.min(...products.map(p => Date.parse(p.currentPriceValidUntil ?? '')).filter(at => at > now.value))
    if (Number.isFinite(next)) timer = globalThis.setTimeout(schedule, Math.min(next - now.value, 2_147_483_647))
  }
  schedule(); cleanup(() => globalThis.clearTimeout(timer))
}, { immediate: true })
const rows = computed(() => props.products.map(p => p.currentPriceValidUntil && Date.parse(p.currentPriceValidUntil) <= now.value ? { ...p, pricingStatus: 'STALE', currentPriceMinor: null } : p))
const columns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { id: 'current', header: t('pricing.currentPrice') }, { id: 'readiness', header: t('pricing.readiness') }, { id: 'type', header: t('pricing.productType') }, { id: 'reference', header: t('userPrices.fixedPrice') }, { id: 'purchase', header: t('pricing.purchase') }, { id: 'renewal', header: t('pricing.renewal') }])
</script>
<template>
  <UTable :data="rows" :columns="columns" class="admin-table">
    <template #name-cell="{ row }"><RouterLink :to="`/admin/pricing/products/${encodeURIComponent(row.original.id)}`">{{ row.original.name }}</RouterLink></template>
    <template #type-cell="{ row }">{{ t(`pricing.types.${row.original.productType}`) }}</template>
    <template #current-cell="{ row }">{{ pricingMoney(row.original.currentPriceMinor ?? (!row.original.policy.dynamicEnabled || row.original.policy.purchasePricingMode === 'FIXED' ? row.original.fixedPriceMinor : null)) }}</template>
    <template #readiness-cell="{ row }">{{ pricingLabel('states', row.original.pricingStatus ?? (row.original.policy.dynamicEnabled && row.original.policy.purchasePricingMode === 'DYNAMIC' ? 'PENDING' : 'FIXED')) }}</template>
    <template #reference-cell="{ row }">{{ pricingMoney(row.original.fixedPriceMinor) }}</template>
    <template #purchase-cell="{ row }">{{ t(`pricing.modes.${row.original.policy.purchasePricingMode}`) }}</template>
    <template #renewal-cell="{ row }">{{ t(`pricing.modes.${row.original.policy.renewalPricingMode}`) }}</template>
    <template #empty>{{ t('pricing.emptyProducts') }}</template>
  </UTable>
</template>
