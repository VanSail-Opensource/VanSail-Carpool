<script setup lang="ts">
import { reactive, shallowRef, watch } from 'vue'
import { pricingAnalysisApi } from '@/api/pricingAnalysis'
import InlineNotice from '@/components/common/InlineNotice.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useI18n } from '@/i18n'
import { moneyFromTxbInput, trafficBytesFromInput, txbInputFromMinor } from '@/utils/format'
const props = defineProps<{ nodeId: string }>()
const { t } = useI18n()
const draft = reactive({ cost: '', days: 30, capacity: '', version: 0, effective: new Date().toISOString().slice(0, 10), reason: '', recordPayment: false, actualPaid: '' })
const busy = shallowRef(false)
const error = shallowRef('')
const saved = shallowRef(false)
const metadataLoading = shallowRef(false)
const metadataFailed = shallowRef(false)
const metadataAttempt = shallowRef(0)
watch(() => [props.nodeId, metadataAttempt.value] as const, async ([node], _, onCleanup) => {
  const controller = new globalThis.AbortController()
  onCleanup(() => controller.abort())
  draft.cost = ''; draft.capacity = ''; draft.days = 30; draft.version = 0
  draft.effective = new Date().toISOString().slice(0, 10); draft.reason = ''; draft.recordPayment = false; draft.actualPaid = ''
  metadataFailed.value = false; metadataLoading.value = false; error.value = ''
  if (!node) return
  metadataLoading.value = true
  try {
    const page = await pricingAnalysisApi.data('pricing/provider_costs', { node }, controller.signal)
    if (controller.signal.aborted) return
    const item = page.items.find(row => row.node_id === node)
    if (item) {
      const version = Number(item.version), days = Number(item.cycle_days)
      if (!Number.isSafeInteger(version) || version < 1 || !Number.isInteger(days) || days < 1 || days > 366 || !/^\d+$/.test(String(item.cycle_cost_minor)) || !/^\d+$/.test(String(item.capacity_bytes)) || typeof item.effective_at !== 'string') throw new Error('invalid_cost_metadata')
      draft.version = version; draft.days = days; draft.cost = txbInputFromMinor(String(item.cycle_cost_minor))
      draft.capacity = String(item.capacity_bytes); draft.effective = item.effective_at.slice(0, 10)
    }
  } catch { if (!controller.signal.aborted) { metadataFailed.value = true; error.value = t('pricing.loadError') } }
  finally { if (!controller.signal.aborted) metadataLoading.value = false }
}, { immediate: true })
async function save(): Promise<void> {
  if (busy.value) return
  const cost = moneyFromTxbInput(draft.cost)
  const capacity = trafficBytesFromInput(draft.capacity)
  const actualPaid = draft.recordPayment ? moneyFromTxbInput(draft.actualPaid) : undefined
  if (metadataLoading.value || metadataFailed.value || !cost || !capacity || BigInt(capacity) <= 0n || (draft.recordPayment && !actualPaid) || !draft.reason.trim() || !props.nodeId || !/^\d{4}-\d{2}-\d{2}$/.test(draft.effective) || draft.effective > new Date().toISOString().slice(0, 10) || !Number.isInteger(draft.days) || draft.days < 1 || draft.days > 366 || !Number.isSafeInteger(draft.version) || draft.version < 0) { error.value = t('pricing.invalid'); return }
  busy.value = true
  error.value = ''; saved.value = false
  try { await pricingAnalysisApi.cost({ nodeId: props.nodeId, cycleCostMinor: cost, cycleDays: draft.days, capacityBytes: capacity, effectiveAt: `${draft.effective}T00:00:00Z`, version: draft.version, reason: draft.reason.trim(), actualPaidMinor: actualPaid }); draft.recordPayment = false; draft.version++; draft.reason = ''; saved.value = true }
  catch { error.value = t('pricing.saveError') }
  finally { busy.value = false }
}
</script>
<template>
  <section class="pricing-costs">
    <h3>{{ t('pricing.providerCosts') }}</h3>
    <p>{{ t('pricing.nodeCostSeparateSave') }}</p>
    <form @submit.prevent="save">
      <fieldset class="pricing-costs__form" :disabled="busy || metadataLoading || metadataFailed">
        <TxbAmountField id="provider-cycle-cost" v-model="draft.cost" :label="t('pricing.cycleCost')" min-minor="0" max-minor="1000000000000" required />
        <UFormField :label="t('pricing.cycleDays')"><UInput v-model.number="draft.days" type="number" :min="1" :max="366" class="w-full" /></UFormField>
        <UFormField :label="t('pricing.capacity')"><UInput v-model="draft.capacity" class="w-full" /></UFormField>
        <UFormField :label="t('pricing.effectiveAt')"><UInput v-model="draft.effective" type="date" class="w-full" /></UFormField>
        <UCheckbox v-model="draft.recordPayment" :label="t('pricing.recordProviderPayment')" />
        <TxbAmountField v-if="draft.recordPayment" id="provider-actual-paid" v-model="draft.actualPaid" :label="t('pricing.actualProviderPayment')" min-minor="0" max-minor="1000000000000" required />
        <UFormField :label="t('pricing.reason')" required><UInput v-model="draft.reason" :maxlength="1000" class="w-full" /></UFormField>
        <UButton type="submit" :label="t('pricing.saveCosts')" :loading="busy || metadataLoading" :disabled="busy || metadataLoading || metadataFailed || !props.nodeId" />
      </fieldset>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <InlineNotice v-if="saved" tone="success">{{ t('pricing.costSaved') }}</InlineNotice>
      <UButton v-if="metadataFailed" type="button" color="neutral" variant="outline" :label="t('common.tryAgain')" @click="metadataAttempt++" />
    </form>
  </section>
</template>
<style scoped>
.pricing-costs { border-top: 1px solid var(--line); padding-top: 1rem; }.pricing-costs h3 { font-weight: 600; }.pricing-costs p { color: var(--text-muted); font-size: .875rem; margin: .5rem 0 1rem; }.pricing-costs__form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; border: 0; padding: 0; margin: 0; min-width: 0; }
@media (max-width: 520px) { .pricing-costs__form { grid-template-columns: 1fr; } }
</style>
