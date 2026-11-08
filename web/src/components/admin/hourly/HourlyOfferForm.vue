<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import * as z from 'zod'
import type { HourlyOffer } from '@/api/hourly'
import type { SquadProduct } from '@/api/types'
import { useI18n } from '@/i18n'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'
const props = defineProps<{ squad: SquadProduct; offer?: HourlyOffer; busy: boolean; activationHours: number }>()
const emit = defineEmits<{ save: [groupId: string, revision: string, enabled: boolean, minor: string] }>()
const { t } = useI18n()
const state = reactive({ enabled: false, price: '0.00' })
watch(() => props.offer, value => Object.assign(state, { enabled: value?.enabled ?? false, price: txbInputFromMinor(value?.hourlyPrice.minor ?? '0') }), { immediate: true })
const schema = computed(() => z.object({
  enabled: z.boolean(), price: z.string().refine(value => {
    const minor = moneyFromTxbInput(value)
    return minor !== '' && BigInt(minor) <= 9223372036854775807n / BigInt(props.activationHours)
  }, t('hourly.admin.invalidPrice')),
}).refine(value => !value.enabled || BigInt(moneyFromTxbInput(value.price) || '0') > 0n, { path: ['price'], message: t('hourly.admin.positivePrice') }))
function save(): void { emit('save', props.squad.groupId, props.offer?.revision ?? '0', state.enabled, moneyFromTxbInput(state.price)) }
</script>

<template>
  <UForm :schema="schema" :state="state" class="hourly-offer-form" @submit="save">
    <div class="hourly-offer-form__name"><strong>{{ squad.name }}</strong><small>{{ squad.groupId }}</small></div>
    <USwitch v-model="state.enabled" :label="t('hourly.admin.offerEnabled')" :aria-label="t('hourly.admin.offerFor', { name: squad.name })" :disabled="busy" />
    <UFormField name="price" :label="t('hourly.admin.hourlyPrice')" required><UInput v-model="state.price" inputmode="decimal" :disabled="busy" class="w-full" /></UFormField>
    <UButton type="submit" color="neutral" variant="outline" :loading="busy" :disabled="busy" :label="t('common.save')" :aria-label="t('hourly.admin.saveOffer', { name: squad.name })" />
  </UForm>
</template>

<style scoped>
.hourly-offer-form { display: grid; grid-template-columns: minmax(10rem, 1fr) minmax(8rem, .6fr) minmax(10rem, .75fr) auto; align-items: center; gap: 1.25rem; padding-block: 1.25rem; border-top: 1px solid var(--border); }
.hourly-offer-form__name { display: grid; gap: .35rem; min-width: 0; }
.hourly-offer-form__name small { color: var(--text-muted); overflow-wrap: anywhere; font-size: .75rem; }
@media (max-width: 760px) { .hourly-offer-form { grid-template-columns: minmax(0, 1fr) auto; gap: 1rem; } .hourly-offer-form__name { grid-column: 1 / -1; } }
</style>
