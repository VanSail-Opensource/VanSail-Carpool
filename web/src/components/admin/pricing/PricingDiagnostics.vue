<script setup lang="ts">
import { useI18n } from '@/i18n'
import PricingDataset from './PricingDataset.vue'
import PricingRecordList from './PricingRecordList.vue'
import PricingResponseCurve from './PricingResponseCurve.vue'
defineProps<{ mode: string; product?: string; user?: string }>()
const { t } = useI18n()
</script>
<template>
  <div class="pricing-diagnostics">
    <PricingDataset v-if="mode === 'product' || mode === 'user'" name="pricing/pricing_decisions" :product="product" :user="user" :title="t('pricing.calculationTrace')">
      <template #default="{ items }"><PricingResponseCurve :record="items[0]" /><PricingRecordList :title="t('pricing.calculationTrace')" :items="items" :fields="['selected_minor','model_version','state','generated_at','trace_json']" /></template>
    </PricingDataset>
    <PricingDataset v-if="mode === 'product' || mode === 'user'" name="pricing/contracts" :product="product" :user="user" :title="t('pricing.renewalBasis')" :fields="['purchase_id','contractual_price_minor','price_limit_minor','recorded_at','policy_version','mode']" />
    <PricingDataset v-if="mode === 'product' || mode === 'user'" name="forecasts/renewal_predictions" :product="product" :user="user" :title="t('pricing.renewalQuotes')" :fields="['purchase_id','user_id','price_minor','probability','contribution','state','health_state','generated_at','trace_json']" />
    <PricingDataset v-if="mode === 'product'" name="pricing/published_prices" :product="product" :title="t('pricing.publishedPrices')" :fields="['price_minor','source','model_version','policy_version','created_at','valid_until']" />
    <PricingDataset v-if="mode === 'user'" name="features/features" :user="user" :title="t('pricing.userFeatures')" :fields="['day','coverage_bps','features_json']" />
    <PricingDataset name="forecasts/model_versions" :product="product" :user="user" :title="t('pricing.modelDiagnostics')" :fields="['id','calibration_status','watermark','diagnostics_json']" />
  </div>
</template>
<style scoped>.pricing-diagnostics { display:grid; gap:1.5rem; min-width:0; }</style>
