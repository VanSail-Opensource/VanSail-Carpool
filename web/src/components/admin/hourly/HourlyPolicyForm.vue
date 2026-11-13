<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import * as z from 'zod'
import type { HourlyPolicy, HourlyPolicySnapshot } from '@/api/hourly'
import { useI18n } from '@/i18n'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'
import { bytesFromSizeInput, bytesPerSecondFromInput, rateInputFromBytesPerSecond, sizeInputFromBytes } from '@/utils/transferUnits'

const props = defineProps<{ snapshot: HourlyPolicySnapshot; busy: boolean; disabled?: boolean }>()
const emit = defineEmits<{ save: [policy: HourlyPolicy] }>()
const { t } = useI18n()
const blocked = computed(() => props.busy || props.disabled)
const state = reactive({ salesEnabled: false, dailyAllowanceBytes: '', reducedRateBytesPerSecond: '', extraPrice: '', activationHours: '', cooldownSeconds: '' })
watch(() => props.snapshot, value => Object.assign(state, {
  salesEnabled: value.policy.salesEnabled,
  dailyAllowanceBytes: sizeInputFromBytes(value.policy.dailyAllowanceBytes),
  reducedRateBytesPerSecond: rateInputFromBytesPerSecond(value.policy.reducedRateBytesPerSecond),
  extraPrice: txbInputFromMinor(value.policy.extraGBPriceTXBMinor),
  activationHours: String(value.policy.activationHours), cooldownSeconds: String(value.policy.cooldownSeconds),
}), { immediate: true })
const schema = computed(() => {
  const integer = (max: bigint, minimum = 0n) => z.string().refine(value => /^\d+$/.test(value) && BigInt(value) >= minimum && BigInt(value) <= max, t('hourly.admin.invalidInteger'))
  const size = (max: bigint) => z.string().refine(value => {
    const bytes = bytesFromSizeInput(value)
    return bytes !== '' && BigInt(bytes) <= max
  }, t('hourly.admin.invalidSize'))
  const speed = (max: bigint) => z.string().refine(value => {
    const bytes = bytesPerSecondFromInput(value)
    return bytes !== '' && BigInt(bytes) <= max
  }, t('hourly.admin.invalidRate'))
  return z.object({
    salesEnabled: z.boolean(), dailyAllowanceBytes: size(9223372036854775807n),
    reducedRateBytesPerSecond: speed(1000000000000n),
    extraPrice: z.string().refine(value => { const minor = moneyFromTxbInput(value); return minor !== '' && BigInt(minor) <= 9223372036854775807n }, t('hourly.admin.invalidPrice')),
    activationHours: integer(2562047n, 1n), cooldownSeconds: integer(9223372036n),
  }).refine(value => !value.salesEnabled || (bytesPerSecondFromInput(value.reducedRateBytesPerSecond) !== '' && BigInt(bytesPerSecondFromInput(value.reducedRateBytesPerSecond)) > 0n), { message: t('hourly.admin.positiveRate'), path: ['reducedRateBytesPerSecond'] })
})
function save(): void {
  if (blocked.value) return
  emit('save', {
    salesEnabled: state.salesEnabled, dailyAllowanceBytes: bytesFromSizeInput(state.dailyAllowanceBytes),
    reducedRateBytesPerSecond: bytesPerSecondFromInput(state.reducedRateBytesPerSecond),
    extraGBPriceTXBMinor: moneyFromTxbInput(state.extraPrice),
    activationHours: Number(state.activationHours), cooldownSeconds: Number(state.cooldownSeconds),
  })
}
</script>

<template>
  <UForm :schema="schema" :state="state" class="hourly-policy-form" @submit="save">
    <div class="hourly-policy-form__wide"><USwitch v-model="state.salesEnabled" :disabled="blocked" :label="t('hourly.admin.salesEnabled')" :description="t('hourly.admin.salesHint')" /></div>
    <UFormField name="dailyAllowanceBytes" :label="t('hourly.admin.allowance')" :description="t('hourly.admin.allowanceHint')" required><UInput v-model="state.dailyAllowanceBytes" autocomplete="off" spellcheck="false" :disabled="blocked" class="w-full" /></UFormField>
    <UFormField name="reducedRateBytesPerSecond" :label="t('hourly.admin.rate')" :description="t('hourly.admin.rateHint')" required><UInput v-model="state.reducedRateBytesPerSecond" autocomplete="off" spellcheck="false" :disabled="blocked" class="w-full" /></UFormField>
    <UFormField name="extraPrice" :label="t('hourly.admin.extraPrice')" :description="t('hourly.admin.extraHint')" required><UInput v-model="state.extraPrice" inputmode="decimal" :disabled="blocked" class="w-full" /></UFormField>
    <UFormField name="activationHours" :label="t('hourly.admin.activationHours')" :description="t('hourly.admin.activationHint')" required><UInput v-model="state.activationHours" inputmode="numeric" :disabled="blocked" class="w-full" /></UFormField>
    <UFormField name="cooldownSeconds" :label="t('hourly.admin.cooldown')" :description="t('hourly.admin.cooldownHint')" required><UInput v-model="state.cooldownSeconds" inputmode="numeric" :disabled="blocked" class="w-full" /></UFormField>
    <div class="hourly-policy-form__wide"><UButton type="submit" :loading="busy" :disabled="blocked" :label="t('hourly.admin.savePolicy')" /></div>
  </UForm>
</template>

<style scoped>
.hourly-policy-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1.25rem 1.75rem; max-width: 58rem; }
.hourly-policy-form__wide { grid-column: 1 / -1; }
@media (max-width: 640px) { .hourly-policy-form { grid-template-columns: minmax(0, 1fr); } }
</style>
