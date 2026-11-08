<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import * as z from 'zod'
import type { HourlyPolicy, HourlyPolicySnapshot } from '@/api/hourly'
import { useI18n } from '@/i18n'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'

const props = defineProps<{ snapshot: HourlyPolicySnapshot; busy: boolean }>()
const emit = defineEmits<{ save: [policy: HourlyPolicy] }>()
const { t } = useI18n()
const state = reactive({ salesEnabled: false, dailyAllowanceBytes: '', reducedRateBytesPerSecond: '', extraPrice: '', activationHours: '', cooldownSeconds: '' })
watch(() => props.snapshot, value => Object.assign(state, {
  ...value.policy, extraPrice: txbInputFromMinor(value.policy.extraGBPriceTXBMinor),
  activationHours: String(value.policy.activationHours), cooldownSeconds: String(value.policy.cooldownSeconds),
}), { immediate: true })
const schema = computed(() => {
  const integer = (max: bigint, minimum = 0n) => z.string().refine(value => /^\d+$/.test(value) && BigInt(value) >= minimum && BigInt(value) <= max, t('hourly.admin.invalidInteger'))
  return z.object({
    salesEnabled: z.boolean(), dailyAllowanceBytes: integer(9223372036854775807n),
    reducedRateBytesPerSecond: integer(1000000000000n),
    extraPrice: z.string().refine(value => { const minor = moneyFromTxbInput(value); return minor !== '' && BigInt(minor) <= 9223372036854775807n }, t('hourly.admin.invalidPrice')),
    activationHours: integer(2562047n, 1n), cooldownSeconds: integer(9223372036n),
  }).refine(value => !value.salesEnabled || /^\d+$/.test(value.reducedRateBytesPerSecond) && BigInt(value.reducedRateBytesPerSecond) > 0n, { message: t('hourly.admin.positiveRate'), path: ['reducedRateBytesPerSecond'] })
})
function save(): void {
  emit('save', {
    salesEnabled: state.salesEnabled, dailyAllowanceBytes: BigInt(state.dailyAllowanceBytes).toString(),
    reducedRateBytesPerSecond: BigInt(state.reducedRateBytesPerSecond).toString(),
    extraGBPriceTXBMinor: moneyFromTxbInput(state.extraPrice),
    activationHours: Number(state.activationHours), cooldownSeconds: Number(state.cooldownSeconds),
  })
}
</script>

<template>
  <UForm :schema="schema" :state="state" class="hourly-policy-form" @submit="save">
    <div class="hourly-policy-form__wide"><USwitch v-model="state.salesEnabled" :disabled="busy" :label="t('hourly.admin.salesEnabled')" :description="t('hourly.admin.salesHint')" /></div>
    <UFormField name="dailyAllowanceBytes" :label="t('hourly.admin.allowance')" :description="t('hourly.admin.allowanceHint')" required><UInput v-model="state.dailyAllowanceBytes" inputmode="numeric" :disabled="busy" class="w-full" /></UFormField>
    <UFormField name="reducedRateBytesPerSecond" :label="t('hourly.admin.rate')" :description="t('hourly.admin.rateHint')" required><UInput v-model="state.reducedRateBytesPerSecond" inputmode="numeric" :disabled="busy" class="w-full" /></UFormField>
    <UFormField name="extraPrice" :label="t('hourly.admin.extraPrice')" :description="t('hourly.admin.extraHint')" required><UInput v-model="state.extraPrice" inputmode="decimal" :disabled="busy" class="w-full" /></UFormField>
    <UFormField name="activationHours" :label="t('hourly.admin.activationHours')" :description="t('hourly.admin.activationHint')" required><UInput v-model="state.activationHours" inputmode="numeric" :disabled="busy" class="w-full" /></UFormField>
    <UFormField name="cooldownSeconds" :label="t('hourly.admin.cooldown')" :description="t('hourly.admin.cooldownHint')" required><UInput v-model="state.cooldownSeconds" inputmode="numeric" :disabled="busy" class="w-full" /></UFormField>
    <div class="hourly-policy-form__wide"><UButton type="submit" :loading="busy" :disabled="busy" :label="t('hourly.admin.savePolicy')" /></div>
  </UForm>
</template>

<style scoped>
.hourly-policy-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1.25rem 1.75rem; max-width: 58rem; }
.hourly-policy-form__wide { grid-column: 1 / -1; }
@media (max-width: 640px) { .hourly-policy-form { grid-template-columns: minmax(0, 1fr); } }
</style>
