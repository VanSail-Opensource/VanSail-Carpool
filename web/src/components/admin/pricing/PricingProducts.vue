<script setup lang="ts">
import { computed } from 'vue'
import type { PricingProduct } from '@/api/pricingAnalysis'
import { useI18n } from '@/i18n'
import { txbInputFromMinor } from '@/utils/format'
defineProps<{ products: PricingProduct[] }>()
const { t } = useI18n()
const columns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { id: 'type', header: t('pricing.productType') }, { id: 'reference', header: t('pricing.amountTxb') }, { id: 'purchase', header: t('pricing.purchase') }, { id: 'renewal', header: t('pricing.renewal') }])
</script>
<template>
  <UTable :data="products" :columns="columns" class="admin-table">
    <template #name-cell="{ row }"><RouterLink :to="`/admin/pricing/products/${encodeURIComponent(row.original.id)}`">{{ row.original.name }}</RouterLink></template>
    <template #type-cell="{ row }">{{ t(`pricing.types.${row.original.productType}`) }}</template>
    <template #reference-cell="{ row }">{{ txbInputFromMinor(row.original.fixedPriceMinor) }}</template>
    <template #purchase-cell="{ row }">{{ t(`pricing.modes.${row.original.policy.purchasePricingMode}`) }}</template>
    <template #renewal-cell="{ row }">{{ t(`pricing.modes.${row.original.policy.renewalPricingMode}`) }}</template>
    <template #empty>{{ t('pricing.emptyProducts') }}</template>
  </UTable>
</template>
