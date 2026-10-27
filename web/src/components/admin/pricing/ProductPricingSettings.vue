<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { pricingApi, type PricingPolicy } from '@/api/pricing'
import SwitchField from '@/components/common/SwitchField.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import { pricingMoney } from '@/utils/pricingDisplay'

const props = defineProps<{ kind: 'combo' | 'payg' | 'addon'; productId: string }>()
const { t } = useI18n()
const policy = shallowRef<PricingPolicy | null>(null)
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef('')
const saved = shallowRef(false)
const reason = shallowRef('')
const purchase = computed({ get: () => policy.value?.purchasePricingMode === 'DYNAMIC', set: value => update({ purchasePricingMode: value ? 'DYNAMIC' : 'FIXED' }) })
const renewal = computed({ get: () => policy.value?.renewalPricingMode === 'DYNAMIC', set: value => update({ renewalPricingMode: value ? 'DYNAMIC' : 'FIXED' }) })
const discounts = computed({ get: () => policy.value?.personalDiscountsEnabled ?? false, set: value => update({ personalDiscountsEnabled: value }) })
function update(values: Partial<PricingPolicy>): void {
  if (!policy.value) return
  policy.value = { ...policy.value, ...values }
  policy.value.dynamicEnabled = policy.value.purchasePricingMode === 'DYNAMIC' || policy.value.renewalPricingMode === 'DYNAMIC' || policy.value.personalDiscountsEnabled
  saved.value = false
}
watch(() => [props.kind, props.productId], async (_, __, onCleanup) => {
  const controller = new globalThis.AbortController()
  onCleanup(() => controller.abort())
  loading.value = true
  policy.value = null
  error.value = ''
  try { const result = await pricingApi.policy(props.kind, props.productId, controller.signal); if (!controller.signal.aborted) policy.value = result }
  catch { if (!controller.signal.aborted) error.value = t('pricing.loadError') }
  finally { if (!controller.signal.aborted) loading.value = false }
}, { immediate: true })
async function save(): Promise<void> {
  if (!policy.value || busy.value) return
  if (!reason.value.trim()) { error.value = t('pricing.invalid'); return }
  busy.value = true
  error.value = ''
  try {
    policy.value = await pricingApi.savePolicy(policy.value, reason.value.trim())
    reason.value = ''
    saved.value = true
  } catch { error.value = t('pricing.saveError') }
  finally { busy.value = false }
}
</script>

<template>
  <fieldset class="pricing-settings" :disabled="busy || loading">
    <legend>{{ t('pricing.settings') }}</legend>
    <p>{{ t('pricing.fixedDefault') }}</p>
    <p v-if="loading" role="status">{{ t('common.loading') }}</p>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <InlineNotice v-if="saved" tone="success">{{ t('pricing.saved') }}</InlineNotice>
    <template v-if="policy">
      <p>{{ t('pricing.reference', { amount: pricingMoney(policy.fixedPriceMinor) }) }}</p>
      <SwitchField :id="`pricing-purchase-${productId}`" v-model="purchase" :label="t('pricing.purchaseDynamic')" />
      <SwitchField v-if="kind === 'combo'" :id="`pricing-renewal-${productId}`" v-model="renewal" :label="t('pricing.renewalDynamic')" :help="t('pricing.legacyProtected')" />
      <SwitchField :id="`pricing-discounts-${productId}`" v-model="discounts" :label="t('pricing.discounts')" />
      <dl class="pricing-settings__limits"><div><dt>{{ t('pricing.maxIncrease') }}</dt><dd>{{ t('pricing.autoPercent', { value: policy.maxIncreaseBps / 100 }) }}</dd></div><div><dt>{{ t('pricing.maxDiscount') }}</dt><dd>{{ t('pricing.autoPercent', { value: policy.maxDiscountBps / 100 }) }}</dd></div></dl>
      <p>{{ t('pricing.automaticLimits') }}</p>
      <UFormField :label="t('pricing.reason')" required><UInput v-model="reason" :maxlength="1000" class="w-full" /></UFormField>
      <UButton type="button" color="neutral" variant="outline" :loading="busy" :disabled="busy || !reason.trim()" :label="t('pricing.savePolicy')" @click="save" />
      <UButton type="button" variant="link" :to="`/admin/pricing/products/${kind}:${encodeURIComponent(productId)}`" :label="t('pricing.details')" />
    </template>
  </fieldset>
</template>

<style scoped>
.pricing-settings { display: grid; gap: 1rem; min-width: 0; margin: 0; padding: 1rem 0; border: 0; border-top: 1px solid var(--line); }
.pricing-settings legend { font-weight: 600; }
.pricing-settings p { margin: 0; color: var(--text-muted); font-size: 0.875rem; }
.pricing-settings__limits { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
@media (max-width: 420px) { .pricing-settings__limits { grid-template-columns: 1fr; } }
</style>
