<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import InlineNotice from '@/components/common/InlineNotice.vue'
import UserPricingAnalysis from './UserPricingAnalysis.vue'
import ProductPricingSettings from './ProductPricingSettings.vue'
import PricingProducts from './PricingProducts.vue'
import PricingHistorySection from './PricingHistorySection.vue'
import PricingOperations from './PricingOperations.vue'
import PricingExports from './PricingExports.vue'
import PricingInterventionForm from './PricingInterventionForm.vue'
import PricingInterventions from './PricingInterventions.vue'
import PricingSummary from './PricingSummary.vue'
import PricingCalibration from './PricingCalibration.vue'
import PricingDataset from './PricingDataset.vue'
import PricingDiagnostics from './PricingDiagnostics.vue'
import { usePricingWorkspace } from './usePricingWorkspace'
const route = useRoute()
const { t } = useI18n()
const mode = computed(() => route.meta.pricingMode as string || 'overview')
const product = computed(() => String(route.params.productId ?? ''))
const user = computed(() => String(route.params.userId ?? ''))
const state = usePricingWorkspace(product, user, mode)
const open = shallowRef<Record<string, boolean>>({})
const refresh = shallowRef(0)
const productKind = computed(() => product.value.split(':')[0] as 'combo' | 'addon' | 'payg')
const productId = computed(() => product.value.split(':').slice(1).join(':'))
const scopeType = computed(() => mode.value === 'product' ? 'product' : mode.value === 'user' ? 'user_product' : 'global')
const scopeId = computed(() => product.value || (user.value ? `${user.value}:combo:` : 'all'))
function toggled(name: string, event: globalThis.Event) { open.value = { ...open.value, [name]: (event.target as globalThis.HTMLDetailsElement).open } }
function changed() { refresh.value++; void state.load() }
</script>
<template>
  <section class="pricing-workspace">
    <header class="pricing-workspace__header">
      <div><h2>{{ t(`pricing.pages.${mode}`) }}</h2><p v-if="product || user"><code>{{ product || user }}</code></p><p v-else>{{ t('pricing.fixedDefault') }}</p></div>
      <nav :aria-label="t('pricing.navigation')"><UButton to="/admin/pricing" color="neutral" variant="ghost" :label="t('pricing.overview')" /><UButton to="/admin/pricing/operations" color="neutral" variant="ghost" :label="t('pricing.operations')" /><UButton to="/admin/pricing/exports" color="neutral" variant="ghost" :label="t('pricing.exports')" /><UButton v-if="mode === 'overview' || mode === 'operations'" color="neutral" variant="outline" :label="t('pricing.refresh')" :loading="state.loading.value" @click="changed" /></nav>
    </header>
    <PricingExports v-if="mode === 'exports'" />
    <template v-else>
      <InlineNotice v-if="state.error.value" tone="warning">{{ state.error.value }}<UButton variant="link" color="neutral" :label="t('common.tryAgain')" @click="state.load()" /></InlineNotice>
      <USkeleton v-if="state.loading.value && !state.dashboard.value && mode === 'overview'" class="h-48" />
      <template v-if="mode === 'overview' && state.dashboard.value">
        <InlineNotice v-if="state.dashboard.value.storage.pressure" tone="warning">{{ t('pricing.storagePressure') }}</InlineNotice>
        <PricingSummary v-if="state.dashboard.value.summary" :value="state.dashboard.value.summary" />
        <PricingProducts :products="state.dashboard.value.products" />
        <div class="row-actions"><UButton v-if="state.cursor.value" color="neutral" variant="outline" :label="t('pricing.firstPage')" @click="state.cursor.value = ''" /><UButton v-if="state.nextCursor.value" color="neutral" variant="outline" :label="t('pricing.nextPage')" @click="state.cursor.value = state.nextCursor.value" /></div>
        <small class="text-muted">{{ t('pricing.storage') }}: {{ formatBytes(state.dashboard.value.storage.pricingBytes) }} / {{ formatBytes(state.dashboard.value.storage.pricingBudgetBytes) }} · {{ t('pricing.databaseStorage') }}: {{ formatBytes(state.dashboard.value.storage.databaseBytes) }}</small>
      </template>
      <ProductPricingSettings v-if="mode === 'product'" :key="product" :kind="productKind" :product-id="productId" />
      <UserPricingAnalysis v-if="mode === 'user'" :key="user" :user-id="user" />
      <PricingOperations v-if="mode === 'operations'" :health="state.health.value" />
      <details v-if="mode !== 'operations'" @toggle="toggled('history',$event)"><summary>{{ t('pricing.history') }}</summary><PricingHistorySection v-if="open.history" :key="`${product}:${user}`" :mode="mode" :product="product" :user="user" :health="state.health.value" :products="state.dashboard.value?.products ?? []" /></details>
      <details @toggle="toggled('calibration',$event)"><summary>{{ t('pricing.calibration') }}</summary><PricingDataset v-if="open.calibration" name="validation/calibration" :product="product" :user="user"><template #default="{ items }"><PricingCalibration :items="items" /></template></PricingDataset></details>
      <details @toggle="toggled('diagnostics',$event)"><summary>{{ t('pricing.modelDiagnostics') }}</summary><PricingDiagnostics v-if="open.diagnostics" :key="refresh" :mode="mode" :product="product" :user="user" /></details>
      <details v-if="mode === 'operations'" @toggle="toggled('renewals',$event)"><summary>{{ t('pricing.deferredRenewals') }}</summary><PricingDataset v-if="open.renewals" :key="refresh" name="forecasts/renewal_predictions" :title="t('pricing.deferredRenewals')" :fields="['purchase_id','user_id','price_minor','state','settle_after','valid_until']" /></details>
      <details @toggle="toggled('interventions',$event)"><summary>{{ t('pricing.interventions') }}</summary><div v-if="open.interventions" :key="refresh" class="pricing-workspace__sections"><PricingInterventionForm :scope-type="scopeType" :scope-id="scopeId" @changed="changed" /><PricingDataset name="operations/admin_overrides" :product="product" :user="user"><template #default="{ items }"><PricingInterventions :items="items" @changed="changed" /></template></PricingDataset><PricingDataset name="operations/health_incidents" :product="product" :user="user" :title="t('pricing.incidents')" :fields="['scope_id','state','started_at','ended_at','reason','acknowledged_by']" /></div></details>
    </template>
  </section>
</template>
<style scoped>
.pricing-workspace { display:grid; gap:1.5rem; min-width:0; }
.pricing-workspace__header { display:flex; align-items:flex-start; justify-content:space-between; flex-wrap:wrap; gap:1rem; min-width:0; }
.pricing-workspace__header nav { display:flex; flex-wrap:wrap; gap:.5rem; }
.pricing-workspace h2 { margin:0; font-size:1.25rem; }.pricing-workspace p { margin:.5rem 0 0; font-size:.85rem; color:var(--text-muted); }.pricing-workspace code { overflow-wrap:anywhere; }
.pricing-workspace details { min-width:0; border-top:1px solid var(--line); padding-top:1rem; }
.pricing-workspace summary { cursor:pointer; font-size:.9rem; font-weight:600; padding:.4rem 0; }
.pricing-workspace details[open] > :not(summary) { margin-top:1rem; }
.pricing-workspace__sections { display:grid; gap:1.5rem; min-width:0; }
@media(max-width:600px) { .pricing-workspace__header nav { width:100%; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); } }
</style>
