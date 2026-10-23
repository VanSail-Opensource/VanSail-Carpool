<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { z } from 'zod'
import { AnimatePresence, motion } from 'motion-v'

import type { Combo, ResetCadence, SquadProduct } from '@/api/types'
import MarkdownEditorField from '@/components/common/MarkdownEditorField.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useI18n } from '@/i18n'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { moneyFromTxbInput, trafficBytesFromInput, txbInputFromMinor } from '@/utils/format'

const props = defineProps<{ combo?: Combo; squads: readonly SquadProduct[]; busy: boolean; error?: string | null }>()
const emit = defineEmits<{ cancel: []; save: [payload: Record<string, unknown>] }>()
const { t } = useI18n()
const resetCadences: ResetCadence[] = ['DAY', 'WEEK', 'MONTH_ROLLING']
const resetItems = computed(() => resetCadences.map((value) => ({ value, label: t(`adminCatalogEditor.reset.${value}`) })))
const draft = reactive({
  whitelistIpQuota: 5, ipLookupQuota: null as number | null, name: '', description: '', priceTxb: '', validityDays: 30, trafficLimitBytes: '',
  resetStrategy: 'MONTH_ROLLING' as ResetCadence, squadProductIds: [] as string[],
  rolloverMinRemainingPercent: 0, active: true,
})
const trafficInvalid = ref(false)
const invalid = ref(false)
const schema = z.object({
  name: z.string().trim().min(1).max(80), description: z.string().max(1000),
  priceTxbMinor: z.string().refine(value => /^\d+$/.test(value) && BigInt(value) <= 1_000_000_000_000n),
  trafficLimitBytes: z.string().refine(value => /^[1-9]\d*$/.test(value) && BigInt(value) <= 9_223_372_036_854_775_807n),
  validityDays: z.number().int().min(1).max(3650),
  ipLookupQuota: z.number().int().min(0).max(1000000).nullable(), whitelistIpQuota: z.number().int().min(0).max(1000),
  resetStrategy: z.enum(['DAY', 'WEEK', 'MONTH_ROLLING']), squadProductIds: z.array(z.string()),
  rolloverMinRemainingBps: z.number().int().min(0).max(10000), active: z.boolean(),
})
const { reducedMotion, offset } = useMotionPreferences()

watch(() => props.combo, (combo) => {
  Object.assign(draft, combo ? {
    name: combo.name,
    description: combo.description,
    priceTxb: txbInputFromMinor(combo.price.minor),
    whitelistIpQuota: combo.whitelistIpQuota ?? 5, ipLookupQuota: combo.ipLookupQuota ?? null, validityDays: combo.validityDays,
    trafficLimitBytes: combo.trafficLimitBytes,
    resetStrategy: combo.resetStrategy,
    squadProductIds: combo.includedSquads.map((squad) => squad.id),
    rolloverMinRemainingPercent: combo.rolloverMinRemainingBps / 100,
    active: combo.active,
  } : {
    whitelistIpQuota: 5, ipLookupQuota: null as number | null, name: '', description: '', priceTxb: '', validityDays: 30,
    trafficLimitBytes: '', resetStrategy: 'MONTH_ROLLING', squadProductIds: [], rolloverMinRemainingPercent: 0, active: true,
  })
}, { immediate: true })

function submit(): void {
  if (props.busy) return
  const priceTxbMinor = moneyFromTxbInput(draft.priceTxb)
  const trafficLimitBytes = trafficBytesFromInput(draft.trafficLimitBytes)
  trafficInvalid.value = trafficLimitBytes === ''
  const result = schema.safeParse({
    name: draft.name, description: draft.description, priceTxbMinor,
    whitelistIpQuota: draft.whitelistIpQuota, ipLookupQuota: draft.ipLookupQuota, validityDays: draft.validityDays, trafficLimitBytes,
    resetStrategy: draft.resetStrategy, squadProductIds: [...draft.squadProductIds],
    rolloverMinRemainingBps: Math.round(draft.rolloverMinRemainingPercent * 100), active: draft.active,
  })
  invalid.value = !result.success
  if (!result.success) return
  draft.trafficLimitBytes = trafficLimitBytes
  emit('save', result.data)
}

function normalizeTrafficLimit(): void {
  const trafficLimitBytes = trafficBytesFromInput(draft.trafficLimitBytes)
  trafficInvalid.value = trafficLimitBytes === ''
  if (trafficLimitBytes) draft.trafficLimitBytes = trafficLimitBytes
}

function setSquad(id: string, selected: boolean): void {
  draft.squadProductIds = selected
    ? [...new Set([...draft.squadProductIds, id])]
    : draft.squadProductIds.filter((value) => value !== id)
}
</script>

<template>
  <USlideover :open="true" :title="combo ? t('adminCatalogEditor.edit', { name: combo.name }) : t('adminCatalogEditor.new')" :description="t('adminCatalog.copy')" :dismissible="!busy" @update:open="!$event && emit('cancel')">
    <template #body>
      <InlineNotice v-if="error || invalid" tone="warning">{{ error ?? t('adminCatalogEditor.invalidFields') }}</InlineNotice>
      <form class="admin-editor-form" @submit.prevent="submit">
        <UFormField name="combo-name" :label="t('adminCatalogEditor.name')" required><UInput v-model.trim="draft.name" class="w-full" :maxlength="80" required /></UFormField>
        <MarkdownEditorField v-model="draft.description" class="catalog-editor__wide" :label="t('adminCatalogEditor.description')" :placeholder="t('adminCatalogEditor.descriptionPlaceholder')" :maxlength="1000" />
        <TxbAmountField id="combo-price" v-model="draft.priceTxb" :label="t('adminCatalogEditor.price')" min-minor="0" max-minor="1000000000000" required />
        <UFormField name="validity-days" :label="t('adminCatalogEditor.validityDays')" required><UInput v-model.number="draft.validityDays" class="w-full" type="number" :min="1" :max="3650" required /></UFormField>
        <UFormField name="ip-lookup-quota" :label="t('ipLookup.quotaHeading')" :hint="t('ipLookup.quotaHelp')"><UInputNumber v-model="draft.ipLookupQuota" class="w-full" :min="0" :max="1000000" :step="1" /></UFormField>
        <UFormField name="whitelist-quota" :label="t('diy.whitelist.comboQuota')"><UInputNumber v-model="draft.whitelistIpQuota" class="w-full" :min="0" :max="1000" :step="1" /></UFormField>
        <UFormField name="traffic-limit" :label="t('adminCatalogEditor.trafficLimit')" :hint="t('adminCatalogEditor.trafficLimitHint')" :error="trafficInvalid ? t('adminCatalogEditor.trafficLimitInvalid') : undefined" required><UInput v-model="draft.trafficLimitBytes" class="w-full" inputmode="text" required @blur="normalizeTrafficLimit" /></UFormField>
        <UFormField name="reset-cadence" :label="t('adminCatalogEditor.resetCadence')"><USelect v-model="draft.resetStrategy" class="w-full" :items="resetItems" /></UFormField>
        <UFormField name="rollover-minimum" :label="t('adminCatalogEditor.rolloverMinimum')" required><UInput v-model.number="draft.rolloverMinRemainingPercent" class="w-full" type="number" :min="0" :max="100" :step="0.01" /></UFormField>
        <fieldset class="catalog-editor__wide squad-picker">
          <legend>{{ t('adminCatalogEditor.includedSquads') }}</legend>
          <motion.div layout class="squad-picker__items">
            <AnimatePresence :initial="false" mode="popLayout">
              <motion.div v-for="squad in squads" :key="squad.id" layout class="squad-picker__option" :initial="reducedMotion ? false : { opacity: 0, y: offset(6) }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.22, ease: 'easeOut' }">
                <span><strong>{{ squad.name }}</strong><small>{{ squad.groupId }}</small></span>
                <UCheckbox :model-value="draft.squadProductIds.includes(squad.id)" :aria-label="squad.name" @update:model-value="setSquad(squad.id, Boolean($event))" />
              </motion.div>
              <motion.p v-if="!squads.length" key="empty-squads" :initial="{ opacity: 0 }" :animate="{ opacity: 1 }">{{ t('adminCatalogEditor.noSquads') }}</motion.p>
            </AnimatePresence>
          </motion.div>
        </fieldset>
        <SwitchField id="combo-active" v-model="draft.active" class="catalog-editor__wide" :label="t('adminCatalogEditor.available')" :help="t('adminCatalogEditor.liveTermsHint')" />
        <UButton class="catalog-editor__wide" type="submit" icon="i-ph-floppy-disk" :loading="busy" :disabled="busy" :label="busy ? t('common.saving') : t('adminCatalogEditor.save')" />
      </form>
    </template>
  </USlideover>
</template>

<style scoped>
.squad-picker { display: grid; gap: 0.5rem; margin: 0; padding: 0; border: 0; }
.squad-picker legend { margin-bottom: 0.3rem; color: var(--text-muted); font-size: 0.78rem; font-weight: 700; }
.squad-picker__items { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.5rem; }
.squad-picker__option { min-height: 54px; display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; padding: 0.55rem; border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface); }
.squad-picker__option strong, .squad-picker__option small { display: block; }
.squad-picker__option small { overflow: hidden; margin-top: 0.2rem; color: var(--text-faint); font-family: var(--font-mono); font-size: 0.6rem; text-overflow: ellipsis; white-space: nowrap; }
</style>
