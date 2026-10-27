<script setup lang="ts">
import { nextTick, watch } from 'vue'
import { useCatalog } from '@/composables/useCatalog'
import CatalogCheckout from '@/components/catalog/CatalogCheckout.vue'
import PricingQuoteNotice from '@/components/common/PricingQuoteNotice.vue'
const state = useCatalog()
watch(state.visibleCombos, async combos => { if (combos[0]) { if (!state.selectedComboId.value) state.selectCombo(combos[0].id); await nextTick(); void state.refreshQuote() } }, { immediate: true })
</script>
<template>
  <div>
    <CatalogCheckout :combo="state.selectedCombo.value" :squads="[]" :coupon="null" :quote="state.quote.value" :quote-ready="state.quoteUsable.value" :quoting="state.quoting.value" :purchasing="state.purchasing.value" :needs-balance="state.needsBalance.value" :error="state.error.value" @confirm="state.confirmPurchase()" />
    <PricingQuoteNotice :state="state.quoteStatus.value" :valid-until="state.quote.value?.quoteValidUntil" @retry="state.refreshQuote()" />
  </div>
</template>
