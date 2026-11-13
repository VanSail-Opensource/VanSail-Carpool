<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import * as z from 'zod'
import type { HourlyOffer } from '@/api/hourly'
import type { SquadProduct } from '@/api/types'
import { useI18n } from '@/i18n'
import { formatMoney, moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'
const props = defineProps<{ squad: SquadProduct; offer?: HourlyOffer; busy: boolean; disabled?: boolean; activationHours: number }>()
const emit = defineEmits<{ save: [groupId: string, revision: string, enabled: boolean, minor: string, requiredHours: number] }>()
const { t } = useI18n()
const blocked = computed(() => props.busy || props.disabled)
const maximumHours = 2_562_047
const state = reactive({ enabled: false, price: '0.00', requiredHours: '1' })
watch(() => [props.offer, props.activationHours] as const, ([offer, activationHours]) => Object.assign(state, {
  enabled: offer?.enabled ?? false,
  price: txbInputFromMinor(offer?.hourlyPrice.minor ?? '0'),
  requiredHours: String(offer?.requiredHours || activationHours),
}), { immediate: true })
const effectiveHours = computed(() => {
  const value = Number(state.requiredHours)
  return Number.isInteger(value) && value >= 1 && value <= maximumHours ? value : props.activationHours
})
const requiredBalance = computed(() => {
  const minor = moneyFromTxbInput(state.price)
  if (minor === '' || effectiveHours.value < 1) return ''
  return formatMoney({ currency: 'TXB', minor: (BigInt(minor) * BigInt(effectiveHours.value)).toString(), display: '' })
})
const priceBoundHours = computed(() => Math.max(1, effectiveHours.value))
const schema = computed(() => z.object({
  enabled: z.boolean(),
  requiredHours: z.string().refine(value => /^\d+$/.test(value) && Number(value) >= 1 && Number(value) <= maximumHours, t('hourly.admin.invalidRequiredHours')),
  price: z.string().refine(value => {
    const minor = moneyFromTxbInput(value)
    return minor !== '' && BigInt(minor) <= 9223372036854775807n / BigInt(priceBoundHours.value)
  }, t('hourly.admin.invalidPrice')),
}).refine(value => !value.enabled || BigInt(moneyFromTxbInput(value.price) || '0') > 0n, { path: ['price'], message: t('hourly.admin.positivePrice') }))
function save(): void { if (blocked.value) return; emit('save', props.squad.groupId, props.offer?.revision ?? '0', state.enabled, moneyFromTxbInput(state.price), Number(state.requiredHours)) }
</script>

<template>
  <UForm :schema="schema" :state="state" class="hourly-offer-form" @submit="save">
    <div class="hourly-offer-form__name"><strong>{{ squad.name }}</strong><small>{{ squad.groupId }}</small></div>
    <USwitch v-model="state.enabled" :label="t('hourly.admin.offerEnabled')" :aria-label="t('hourly.admin.offerFor', { name: squad.name })" :disabled="blocked" />
    <UFormField name="price" :label="t('hourly.admin.hourlyPrice')" required><UInput v-model="state.price" inputmode="decimal" :disabled="blocked" class="w-full" /></UFormField>
    <UFormField name="requiredHours" :label="t('hourly.admin.requiredHours')" :hint="requiredBalance ? t('hourly.admin.requiredHoursHint', { hours: effectiveHours, amount: requiredBalance }) : undefined" required><UInput v-model.trim="state.requiredHours" inputmode="numeric" :disabled="blocked" class="w-full" /></UFormField>
    <UButton type="submit" color="neutral" variant="outline" :loading="busy" :disabled="blocked" :label="t('common.save')" :aria-label="t('hourly.admin.saveOffer', { name: squad.name })" />
  </UForm>
</template>

<style scoped>
.hourly-offer-form { display: grid; grid-template-columns: minmax(9rem, 1fr) minmax(7rem, .5fr) minmax(9rem, .6fr) minmax(10rem, .7fr) auto; align-items: start; gap: 1.1rem; padding-block: 1.25rem; border-top: 1px solid var(--border); }
.hourly-offer-form__name { display: grid; gap: .35rem; min-width: 0; padding-top: 1.55rem; }
.hourly-offer-form__name small { color: var(--text-muted); overflow-wrap: anywhere; font-size: .75rem; }
@media (max-width: 900px) { .hourly-offer-form { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 1rem; } .hourly-offer-form__name { grid-column: 1 / -1; padding-top: 0; } }
@media (max-width: 560px) { .hourly-offer-form { grid-template-columns: minmax(0, 1fr); } }
</style>
