<script setup lang="ts">
import { reactive, shallowRef } from 'vue'
import { pricingAnalysisApi } from '@/api/pricingAnalysis'
import InlineNotice from '@/components/common/InlineNotice.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useI18n } from '@/i18n'
import { moneyFromTxbInput, trafficBytesFromInput } from '@/utils/format'
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const draft = reactive({ node: '', cost: '', days: 30, capacity: '', version: 0, effective: new Date().toISOString().slice(0, 10), reason: '', recordPayment: false, actualPaid: '' })
const busy = shallowRef(false)
const error = shallowRef('')
async function save(): Promise<void> {
  const cost = moneyFromTxbInput(draft.cost)
  const capacity = trafficBytesFromInput(draft.capacity)
  if (!cost || !capacity || !draft.reason || !draft.node || !Number.isInteger(draft.days) || draft.days < 1 || draft.days > 366 || !Number.isInteger(draft.version) || draft.version < 0) { error.value = t('pricing.invalid'); return }
  busy.value = true
  error.value = ''
  try { await pricingAnalysisApi.cost({ nodeId: draft.node, cycleCostMinor: cost, cycleDays: draft.days, capacityBytes: capacity, effectiveAt: `${draft.effective}T00:00:00Z`, version: draft.version, reason: draft.reason, actualPaidMinor: draft.recordPayment ? moneyFromTxbInput(draft.actualPaid) ?? undefined : undefined }); draft.recordPayment = false; draft.version++; draft.reason = ''; emit('changed') }
  catch { error.value = t('pricing.saveError') }
  finally { busy.value = false }
}
</script>
<template>
  <section class="pricing-costs">
    <h3>{{ t('pricing.providerCosts') }}</h3>
    <form class="pricing-costs__form" @submit.prevent="save">
      <UFormField :label="t('pricing.node')"><UInput v-model="draft.node" class="w-full" /></UFormField>
      <TxbAmountField id="provider-cycle-cost" v-model="draft.cost" :label="t('pricing.cycleCost')" min-minor="0" max-minor="1000000000000" required />
      <UFormField :label="t('pricing.cycleDays')"><UInput v-model.number="draft.days" type="number" :min="1" :max="366" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.capacity')"><UInput v-model="draft.capacity" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.effectiveAt')"><UInput v-model="draft.effective" type="date" class="w-full" /></UFormField>
      <UCheckbox v-model="draft.recordPayment" :label="t('pricing.recordProviderPayment')" />
      <TxbAmountField v-if="draft.recordPayment" id="provider-actual-paid" v-model="draft.actualPaid" :label="t('pricing.actualProviderPayment')" min-minor="0" max-minor="1000000000000" required />
      <UFormField :label="t('pricing.expectedVersion')"><UInput v-model.number="draft.version" type="number" :min="0" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.reason')" required><UInput v-model="draft.reason" :maxlength="1000" class="w-full" /></UFormField>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <UButton type="submit" :label="t('pricing.saveCosts')" :loading="busy" :disabled="busy" />
    </form>
  </section>
</template>
<style scoped>
.pricing-costs h3 { cursor: pointer; font-weight: 600; padding: 1rem 0; }.pricing-costs__form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
@media (max-width: 520px) { .pricing-costs__form { grid-template-columns: 1fr; } }
</style>
