<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from 'vue'
import { pricingAnalysisApi } from '@/api/pricingAnalysis'
import InlineNotice from '@/components/common/InlineNotice.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useI18n } from '@/i18n'
import { moneyFromTxbInput } from '@/utils/format'
const props = defineProps<{ scopeType?: string; scopeId?: string }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const draft = reactive({ scopeType: props.scopeType ?? 'global', scopeId: props.scopeId ?? 'all', command: 'FREEZE', reason: '', price: '', discountBps: 0, budget: '', priceVersion: '', hours: 24 })
const busy = shallowRef(false)
const error = shallowRef('')
const result = shallowRef('')
const version = shallowRef<number | null>(null)
const commands = ['FREEZE', 'DISABLE', 'MANUAL_RATE', 'PERSONAL_DISCOUNT', 'PAUSE_ADMISSIONS', 'MAINTENANCE', 'SUPPRESS_TELEMETRY', 'RETRY_MODEL', 'RECALIBRATE', 'RESTORE_PRICE', 'DISABLE_EXPERIMENTS', 'HOLD_RENEWAL', 'ACK_INCIDENT']
const commandItems = computed(() => commands.map(value => ({ value, label: t(`pricing.commands.${value}`) })))
const scopeItems = computed(() => ['global', 'product', 'node', 'machine', 'user_product', 'purchase', 'model', 'incident'].map(value => ({ value, label: t(`pricing.scopes.${value}`) })))
watch(() => [draft.scopeType, draft.scopeId], async (_, __, onCleanup) => {
  let cancelled = false
  onCleanup(() => { cancelled = true })
  version.value = null
  try { const value = await pricingAnalysisApi.version(draft.scopeType, draft.scopeId); if (!cancelled) version.value = value.version }
  catch { if (!cancelled) error.value = t('pricing.loadError') }
}, { immediate: true })
async function submit(): Promise<void> {
  if (version.value === null || busy.value || !draft.reason.trim()) return
  const price = moneyFromTxbInput(draft.price) ?? '0'
  const budget = moneyFromTxbInput(draft.budget) ?? '0'
  if (!Number.isInteger(draft.hours) || draft.hours < 1 || draft.hours > 168 || !Number.isInteger(draft.discountBps) || draft.discountBps < 0 || draft.discountBps > 10000) { error.value = t('pricing.invalid'); return }
  busy.value = true
  error.value = ''
  try {
    const item = await pricingAnalysisApi.intervene({ scopeType: draft.scopeType, scopeId: draft.scopeId, command: draft.command, reason: draft.reason.trim(), priceMinor: price, discountBps: draft.discountBps, budgetMinor: budget, priceVersion: draft.priceVersion, expectedVersion: version.value, expiresAt: new Date(Date.now() + draft.hours * 3600000).toISOString() })
    result.value = t(`pricing.states.${item.status}`)
    version.value = (await pricingAnalysisApi.version(draft.scopeType, draft.scopeId)).version
    draft.reason = ''
    emit('changed')
  } catch { error.value = t('pricing.saveError') }
  finally { busy.value = false }
}
</script>
<template>
  <section class="pricing-intervention">
    <h3>{{ t('pricing.intervene') }}</h3>
    <form class="pricing-intervention__form" @submit.prevent="submit">
      <UFormField :label="t('pricing.command')"><USelect v-model="draft.command" :items="commandItems" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.scope')"><USelect v-model="draft.scopeType" :items="scopeItems" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.scopeId')"><UInput v-model="draft.scopeId" :maxlength="256" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.hours')"><UInput v-model.number="draft.hours" type="number" :min="1" :max="168" class="w-full" /></UFormField>
      <TxbAmountField v-if="draft.command === 'MANUAL_RATE'" id="manual-pricing-rate" v-model="draft.price" :label="t('pricing.manualRate')" min-minor="0" max-minor="1000000000000" required />
      <template v-if="draft.command === 'PERSONAL_DISCOUNT'">
        <UFormField :label="t('pricing.campaignDiscount')"><UInput v-model.number="draft.discountBps" type="number" :min="0" :max="10000" class="w-full" /></UFormField>
        <TxbAmountField id="discount-budget" v-model="draft.budget" :label="t('pricing.budget')" min-minor="1" max-minor="1000000000000" required />
      </template>
      <UFormField v-if="draft.command === 'RESTORE_PRICE'" :label="t('pricing.priceVersion')"><UInput v-model="draft.priceVersion" class="w-full" /></UFormField>
      <UFormField class="pricing-intervention__wide" :label="t('pricing.reason')" required><UTextarea v-model="draft.reason" :maxlength="1000" class="w-full" required /></UFormField>
      <InlineNotice v-if="error" class="pricing-intervention__wide" tone="warning">{{ error }}</InlineNotice>
      <InlineNotice v-if="result" class="pricing-intervention__wide">{{ result }}</InlineNotice>
      <UButton type="submit" :label="t('pricing.apply')" :disabled="busy || version === null || !draft.reason.trim()" :loading="busy" />
    </form>
  </section>
</template>
<style scoped>
.pricing-intervention { padding: 1rem 0; border-top: 1px solid var(--line); }
.pricing-intervention h3 { cursor: pointer; font-weight: 600; }
.pricing-intervention__form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; margin-top: 1rem; }
.pricing-intervention__wide { grid-column: 1 / -1; }
@media (max-width: 520px) { .pricing-intervention__form { grid-template-columns: 1fr; } }
</style>
