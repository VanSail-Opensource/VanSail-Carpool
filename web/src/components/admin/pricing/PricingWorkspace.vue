<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import InlineNotice from '@/components/common/InlineNotice.vue'
import UserPricingAnalysis from './UserPricingAnalysis.vue'
import ProductPricingSettings from './ProductPricingSettings.vue'
import PricingProducts from './PricingProducts.vue'
import PricingHistory from './PricingHistory.vue'
import PricingOperations from './PricingOperations.vue'
import PricingExports from './PricingExports.vue'
import PricingInterventionForm from './PricingInterventionForm.vue'
import PricingInterventions from './PricingInterventions.vue'
import PricingRecordList from './PricingRecordList.vue'
import PricingCostEditor from './PricingCostEditor.vue'
import PricingSummary from './PricingSummary.vue'
import PricingCalibration from './PricingCalibration.vue'
import PricingResponseCurve from './PricingResponseCurve.vue'
import { usePricingWorkspace } from './usePricingWorkspace'

const route = useRoute()
const { t } = useI18n()
const mode = computed(() => route.meta.pricingMode as string || 'overview')
const product = computed(() => String(route.params.productId ?? ''))
const user = computed(() => String(route.params.userId ?? ''))
const state = usePricingWorkspace(product, user, mode)
const selectedNode = shallowRef('')
watch(() => state.dashboard.value?.health, nodes => { if (!selectedNode.value && nodes?.length) selectedNode.value = nodes[0]!.nodeId }, { immediate: true })
const nodeItems = computed(() => state.dashboard.value?.health.map(node => ({ value: node.nodeId, label: node.nodeId })) ?? [])
const productKind = computed(() => product.value.split(':')[0] as 'combo' | 'addon' | 'payg')
const productId = computed(() => product.value.split(':').slice(1).join(':'))
const records = (name: string) => state.data.value[name] ?? []
const events = computed(() => [...records('operations/health_incidents'), ...records('operations/admin_overrides'), ...records('operations/policy_events'), ...records('pricing/provider_cost_versions').map(c => ({ ...c, event_type: 'COST', created_at: c.recorded_at })), ...records('forecasts/model_versions').map(m => ({ ...m, event_type: 'MODEL' }))])
const scopeType = computed(() => mode.value === 'product' ? 'product' : mode.value === 'user' ? 'user_product' : 'global')
const scopeId = computed(() => product.value || (user.value ? `${user.value}:combo:` : 'all'))
const entityType = computed(() => mode.value === 'product' ? 'product' : mode.value === 'user' ? 'user_node' : 'node')
const entityId = computed(() => mode.value === 'product' ? product.value : user.value && selectedNode.value ? `${user.value}:${selectedNode.value}` : selectedNode.value)
const sections = computed(() => mode.value === 'exports' ? [] : [...(mode.value === 'overview' ? ['summary'] : []), ...(mode.value !== 'operations' ? ['history'] : []), 'calibration', ...(mode.value === 'operations' || mode.value === 'overview' ? ['operations'] : []), 'interventions'])
const deferred = computed(() => records('forecasts/renewal_predictions').filter(q => q.state === 'DEFERRED_HEALTH' || q.state === 'AUTHORIZED'))
</script>
<template>
  <section class="pricing-workspace">
    <header class="pricing-workspace__header"><div><h2>{{ t(`pricing.pages.${mode}`) }}</h2><p v-if="product || user"><code>{{ product || user }}</code></p><p v-else>{{ t('pricing.fixedDefault') }}</p></div><nav class="pricing-workspace__nav" :aria-label="t('pricing.navigation')"><UButton to="/admin/pricing" color="neutral" variant="ghost" :label="t('pricing.overview')" /><UButton to="/admin/pricing/operations" color="neutral" variant="ghost" :label="t('pricing.operations')" /><UButton to="/admin/pricing/exports" color="neutral" variant="ghost" :label="t('pricing.exports')" /><UButton color="neutral" variant="outline" :label="t('pricing.refresh')" :loading="state.loading.value" @click="state.load" /></nav></header>
    <nav class="row-actions" :aria-label="t('pricing.navigation')"><UButton v-for="section in sections" :key="section" color="neutral" variant="ghost" :href="`#pricing-${section}`" :label="t(`pricing.${section}`)" /></nav>
    <UserPricingAnalysis v-if="mode === 'user'" :key="user" :user-id="user" />
    <PricingExports v-if="mode === 'exports'" />
    <template v-else>
      <p v-if="state.loading.value && !state.dashboard.value" role="status">{{ t('common.loading') }}</p>
      <InlineNotice v-if="state.error.value" tone="warning">{{ state.error.value }}</InlineNotice>
      <template v-if="state.dashboard.value">
        <InlineNotice v-if="state.dashboard.value.storage.pressure" tone="warning">{{ t('pricing.storagePressure') }}</InlineNotice>
        <dl v-if="mode === 'overview'" class="pricing-workspace__summary"><div><dt>{{ t('pricing.storage') }}</dt><dd>{{ formatBytes(state.dashboard.value.storage.pricingBytes) }} / {{ formatBytes(state.dashboard.value.storage.pricingBudgetBytes) }}</dd></div><div><dt>{{ t('pricing.databaseStorage') }}</dt><dd>{{ formatBytes(state.dashboard.value.storage.databaseBytes) }} / {{ formatBytes(state.dashboard.value.storage.databaseTargetBytes) }}</dd></div></dl>
        <PricingSummary id="pricing-summary" v-if="mode === 'overview' && state.dashboard.value.summary" :value="state.dashboard.value.summary" />
        <template v-if="mode === 'overview'"><PricingProducts :products="state.dashboard.value.products" /><div class="row-actions"><UButton v-if="state.cursor.value" variant="outline" color="neutral" :label="t('pricing.firstPage')" @click="state.cursor.value = ''" /><UButton v-if="state.nextCursor.value" variant="outline" color="neutral" :label="t('pricing.nextPage')" @click="state.cursor.value = state.nextCursor.value" /></div></template>
        <ProductPricingSettings v-if="mode === 'product'" :key="product" :kind="productKind" :product-id="productId" />
        <template v-if="mode !== 'operations'">
          <UFormField v-if="mode !== 'product'" :label="t('pricing.node')"><USelect v-model="selectedNode" :items="nodeItems" class="w-full max-w-lg" /></UFormField>
          <PricingHistory id="pricing-history" :entity-type="entityType" :entity-id="entityId" :events="events" :provider-cycle="records('pricing/provider_costs').find(c => c.node_id === selectedNode)" />
        </template>
        <PricingCalibration id="pricing-calibration" :items="records('validation/calibration')" />
        <PricingResponseCurve v-if="mode === 'product' || mode === 'user'" :record="mode === 'product' ? records('pricing/pricing_decisions')[0] : records('forecasts/renewal_predictions')[0]" />
        <PricingRecordList v-if="mode === 'user' || mode === 'product'" :title="t('pricing.renewalBasis')" :items="records('pricing/contracts')" :fields="['purchase_id', 'contractual_price_minor', 'price_limit_minor', 'recorded_at', 'policy_version', 'mode']" />
        <PricingOperations id="pricing-operations" v-if="mode === 'operations' || mode === 'overview'" :health="state.dashboard.value.health" />
        <PricingCostEditor v-if="mode === 'operations'" @changed="state.load" />
        <PricingRecordList v-if="mode === 'operations'" :title="t('pricing.providerCosts')" :items="records('pricing/provider_costs')" :fields="['node_id', 'cycle_cost_minor', 'cycle_days', 'capacity_bytes', 'effective_at', 'version']" />
        <PricingRecordList v-if="mode === 'operations'" :title="t('pricing.deferredRenewals')" :items="deferred" :fields="['purchase_id', 'user_id', 'price_minor', 'state', 'settle_after', 'valid_until']" />
        <PricingRecordList v-if="mode === 'user'" :title="t('pricing.userFeatures')" :items="records('features/features')" :fields="['day', 'coverage_bps', 'features_json']" />
        <PricingRecordList v-if="mode === 'product' || mode === 'user'" :title="t('pricing.renewalQuotes')" :items="records('forecasts/renewal_predictions')" :fields="['purchase_id', 'user_id', 'price_minor', 'probability', 'contribution', 'state', 'health_state', 'generated_at', 'trace_json']" />
        <PricingRecordList v-if="mode === 'product'" :title="t('pricing.publishedPrices')" :items="records('pricing/published_prices')" :fields="['id', 'price_minor', 'source', 'model_version', 'policy_version', 'created_at', 'valid_until']" />
        <PricingRecordList v-if="mode === 'product' || mode === 'user'" :title="t('pricing.calculationTrace')" :items="records('pricing/pricing_decisions')" :fields="['selected_minor', 'model_version', 'state', 'generated_at', 'trace_json']" />
        <section><h3>{{ t('pricing.modelDiagnostics') }}</h3><PricingRecordList :title="t('pricing.modelDiagnostics')" :items="records('forecasts/model_versions')" :fields="['id', 'calibration_status', 'watermark', 'diagnostics_json']" /></section>
        <PricingInterventionForm :key="scopeId" :scope-type="scopeType" :scope-id="scopeId" @changed="state.load" />
        <PricingInterventions id="pricing-interventions" :items="records('operations/admin_overrides')" @changed="state.load" />
        <PricingRecordList :title="t('pricing.incidents')" :items="records('operations/health_incidents')" :fields="['scope_id', 'state', 'started_at', 'ended_at', 'reason', 'acknowledged_by']" />
      </template>
    </template>
  </section>
</template>
<style scoped>
.pricing-workspace { grid-template-columns: minmax(0, 1fr); display: grid; gap: 2rem; min-width: 0; }.pricing-workspace__nav { display: flex; flex-wrap: wrap; gap: .5rem; min-width: 0; }.pricing-workspace__header { min-width: 0; display: flex; align-items: flex-start; flex-wrap: wrap; justify-content: space-between; gap: 1rem; }.pricing-workspace h2 { margin: 0; font-size: 1.25rem; }.pricing-workspace__header p { max-width: 55rem; margin: .5rem 0 0; color: var(--text-muted); font-size: .85rem; }.pricing-workspace__header code { overflow-wrap: anywhere; }.pricing-workspace__summary { display: flex; flex-wrap: wrap; gap: 1rem 3rem; margin: 0; }.pricing-workspace__summary dt { color: var(--text-muted); font-size: .8rem; }.pricing-workspace__summary dd { margin: .4rem 0 0; font-variant-numeric: tabular-nums; }.pricing-workspace h3 { cursor: pointer; font-weight: 600; }
@media (max-width: 600px) { .pricing-workspace { gap: 1.5rem; }.pricing-workspace__header nav { width: 100%; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
