<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { PricingProduct } from '@/api/pricingAnalysis'
import { useI18n } from '@/i18n'
import { pricingLabel, pricingMoney } from '@/utils/pricingDisplay'
const props = defineProps<{ products: PricingProduct[] }>()
const { t } = useI18n()
const now = shallowRef(Date.now())
const search = shallowRef('')
const filter = shallowRef('all')
const filters = computed(() => ['all', 'dynamic', 'fixed'].map(value => ({ value, label: t(`pricing.filters.${value}`) })))
const getRowId = (row: PricingProduct) => row.id
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
const visible = computed(() => {
  const term = search.value.trim().toLocaleLowerCase()
  return rows.value.filter(p => (!term || `${p.name} ${p.id}`.toLocaleLowerCase().includes(term)) && (filter.value === 'all' || p.policy.dynamicEnabled === (filter.value === 'dynamic')))
    .sort((a, b) => Number(b.policy.dynamicEnabled) - Number(a.policy.dynamicEnabled) || a.name.localeCompare(b.name))
})
const columns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { id: 'current', header: t('pricing.currentPrice') }, { id: 'readiness', header: t('pricing.readiness') }, { id: 'modes', header: t('pricing.settings') }])
</script>
<template>
  <section class="pricing-products">
    <div class="pricing-products__toolbar">
      <div><h3>{{ t('pricing.products') }}</h3><p>{{ t('pricing.productCount', { shown: visible.length, total: products.length }) }}</p></div>
      <div class="pricing-products__filters">
        <UInput v-model="search" type="search" icon="i-ph-magnifying-glass" :placeholder="t('pricing.searchProducts')" :aria-label="t('pricing.searchProducts')" />
        <USelect v-model="filter" :items="filters" :aria-label="t('pricing.filterProducts')" />
      </div>
    </div>
    <UTable :data="visible" :columns="columns" :get-row-id="getRowId" class="admin-table" :ui="{ td: 'align-top whitespace-normal', th: 'whitespace-nowrap' }">
      <template #name-cell="{ row }"><div class="pricing-products__identity"><RouterLink :to="`/admin/pricing/products/${encodeURIComponent(row.original.id)}`">{{ row.original.name }}</RouterLink><small>{{ t(`pricing.types.${row.original.productType}`) }}</small></div></template>
      <template #current-cell="{ row }"><div class="pricing-products__price"><strong>{{ pricingMoney(row.original.currentPriceMinor ?? (!row.original.policy.dynamicEnabled || row.original.policy.purchasePricingMode === 'FIXED' ? row.original.fixedPriceMinor : null)) }}</strong><small>{{ t('pricing.reference', { amount: pricingMoney(row.original.fixedPriceMinor) }) }}</small></div></template>
      <template #readiness-cell="{ row }"><div class="pricing-products__readiness"><span class="status-badge" :class="{ 'status-badge--success': row.original.pricingStatus === 'PUBLISHED', 'status-badge--warning': ['PENDING', 'STALE', 'UNAVAILABLE'].includes(row.original.pricingStatus ?? '') }">{{ pricingLabel('states', row.original.pricingStatus ?? 'FIXED') }}</span><small v-if="row.original.pricingReason">{{ pricingLabel('reasons', row.original.pricingReason) }}</small></div></template>
      <template #modes-cell="{ row }"><div class="pricing-products__modes"><span>{{ t('pricing.purchase') }}: {{ t(`pricing.modes.${row.original.policy.purchasePricingMode}`) }}</span><small v-if="row.original.productType === 'combo'">{{ t('pricing.renewal') }}: {{ t(`pricing.modes.${row.original.policy.renewalPricingMode}`) }}</small></div></template>
      <template #empty>{{ t(products.length ? 'pricing.noMatchingProducts' : 'pricing.emptyProducts') }}</template>
    </UTable>
  </section>
</template>
<style scoped>
.pricing-products { min-width: 0; }
.pricing-products__toolbar { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 1rem; margin-bottom: 1rem; }
.pricing-products__toolbar h3 { margin: 0; font-size: 1rem; }
.pricing-products__toolbar p { margin: .25rem 0 0; font-size: .8rem; color: var(--text-muted); }
.pricing-products__filters { display: flex; flex-wrap: wrap; gap: .5rem; }
.pricing-products__identity, .pricing-products__price, .pricing-products__modes, .pricing-products__readiness { display: grid; justify-items: start; gap: .35rem; }
.pricing-products__identity { min-width: 9rem; max-width: 20rem; overflow-wrap: anywhere; }
.pricing-products__identity a { color: var(--text); font-weight: 550; text-decoration: underline; text-decoration-color: var(--line-strong); text-underline-offset: .2em; }
.pricing-products__readiness { min-width: 12rem; max-width: 24rem; }
.pricing-products small { color: var(--text-muted); font-size: .75rem; line-height: 1.5; }
.pricing-products__price { min-width: 9rem; font-variant-numeric: tabular-nums; }
.pricing-products__price strong { font-weight: 600; }
.pricing-products__modes { min-width: 9rem; }
@media(max-width: 600px) { .pricing-products__filters { width: 100%; display: grid; grid-template-columns: minmax(0, 1fr); } }
</style>
