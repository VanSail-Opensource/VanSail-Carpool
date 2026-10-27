<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { pricingAnalysisApi, type PricingHealth } from '@/api/pricingAnalysis'
import { useI18n } from '@/i18n'
import InlineNotice from '@/components/common/InlineNotice.vue'
import PricingHistory from './PricingHistory.vue'
import { usePricingDataset } from './usePricingDataset'
const props = defineProps<{ mode: string; product: string; user: string; health: PricingHealth[] }>()
const { t } = useI18n()
const nodes = shallowRef<PricingHealth[]>(props.health)
const selected = shallowRef('')
const error = shallowRef('')
const abort = new globalThis.AbortController()
if (props.mode !== 'product' && !nodes.value.length) void pricingAnalysisApi.health(abort.signal).then(result => { if (!abort.signal.aborted) nodes.value = result.health }).catch(() => { if (!abort.signal.aborted) error.value = t('pricing.loadError') })
watch(() => props.health, values => { if (values.length) nodes.value = values })
watch(nodes, values => { if (!values.some(n => n.nodeId === selected.value)) selected.value = values[0]?.nodeId ?? '' }, { immediate: true })
const scopeProduct = () => props.product || undefined
const scopeUser = () => props.user || undefined
const incidents = usePricingDataset('operations/health_incidents', scopeProduct, scopeUser)
const changes = usePricingDataset('operations/policy_events', scopeProduct, scopeUser)
const costs = usePricingDataset('pricing/provider_cost_versions', scopeProduct, scopeUser)
const providers = usePricingDataset('pricing/provider_costs', scopeProduct, scopeUser)
const entityType = computed(() => props.mode === 'product' ? 'product' : props.mode === 'user' ? 'user_node' : 'node')
const entityId = computed(() => props.mode === 'product' ? props.product : props.user && selected.value ? `${props.user}:${selected.value}` : selected.value)
const events = computed(() => [...incidents.items.value, ...changes.items.value, ...costs.items.value.map(c => ({ ...c, event_type: 'COST', created_at: c.recorded_at }))])
onScopeDispose(() => abort.abort())
</script>
<template>
  <div class="pricing-history-section">
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <UFormField v-if="mode !== 'product'" :label="t('pricing.node')"><USelect v-model="selected" :items="nodes.map(node => ({ label: node.nodeId, value: node.nodeId }))" class="w-full max-w-lg" /></UFormField>
    <PricingHistory :entity-type="entityType" :entity-id="entityId" :events="events" :provider-cycle="providers.items.value.find(c => c.node_id === selected)" />
    <InlineNotice v-if="incidents.error.value || changes.error.value || costs.error.value || providers.error.value" tone="warning">{{ t('pricing.eventsUnavailable') }}</InlineNotice>
  </div>
</template>
<style scoped>.pricing-history-section { display:grid; gap:1rem; min-width:0; }</style>
