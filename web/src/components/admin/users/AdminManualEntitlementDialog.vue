<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { AdminEntitlementCreateRequest, Combo, SquadProduct } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { fromLocalDateTime, toLocalDateTime } from './adminUserFormat'

const props = defineProps<{
  open: boolean
  combos: Combo[]
  squads: SquadProduct[]
  busy: boolean
  optionsLoading: boolean
  error?: string | null
}>()
const emit = defineEmits<{ 'update:open': [value: boolean]; save: [body: AdminEntitlementCreateRequest] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const draft = reactive({
  comboId: '', validFrom: '', validUntil: '', trafficLimitBytes: '',
  resetStrategy: 'MONTH_ROLLING' as AdminEntitlementCreateRequest['resetStrategy'],
  squadUuids: [] as string[], reason: '',
})

const comboItems = computed(() => props.combos.map(combo => ({ label: combo.name, value: combo.id })))
const squadItems = computed(() => props.squads.map(squad => ({ label: squad.name, value: squad.groupId })))
const cadenceItems = computed(() => ['DAY', 'WEEK', 'MONTH_ROLLING']
  .map(value => ({ value, label: t(`adminUserProfile.cadence.${value}`) })))
const selectedCombo = computed(() => props.combos.find(combo => combo.id === draft.comboId) ?? null)
const datesValid = computed(() => {
  const from = new Date(draft.validFrom).getTime()
  const until = new Date(draft.validUntil).getTime()
  return Number.isFinite(from) && Number.isFinite(until) && until > from
})
const trafficError = computed(() => draft.trafficLimitBytes && !/^[1-9]\d*$/.test(draft.trafficLimitBytes)
  ? t('adminUserProfile.invalidTrafficBytes') : undefined)
const canSave = computed(() => Boolean(selectedCombo.value && datesValid.value && !trafficError.value
  && /^[1-9]\d*$/.test(draft.trafficLimitBytes) && draft.reason.trim().length >= 3))

function applyComboDefaults(combo: Combo): void {
  draft.trafficLimitBytes = combo.trafficLimitBytes
  draft.resetStrategy = combo.resetStrategy
  if (!draft.validUntil) draft.validUntil = toLocalDateTime(new Date(Date.now() + combo.validityDays * 86_400_000).toISOString())
}

watch(() => props.open, (open) => {
  if (!open) return
  const combo = props.combos[0] ?? null
  Object.assign(draft, {
    comboId: combo?.id ?? '', validFrom: toLocalDateTime(new Date().toISOString()), validUntil: '',
    trafficLimitBytes: '', resetStrategy: 'MONTH_ROLLING' as const, squadUuids: [], reason: '',
  })
  if (combo) applyComboDefaults(combo)
}, { immediate: true })

watch(() => draft.comboId, (comboId) => {
  const combo = props.combos.find(value => value.id === comboId)
  if (combo) applyComboDefaults(combo)
})

watch(() => props.combos, (combos) => {
  if (!props.open || draft.comboId || !combos.length) return
  draft.comboId = combos[0]!.id
})

function submit(): void {
  if (props.busy || !canSave.value) return
  emit('save', {
    comboId: draft.comboId,
    validFrom: fromLocalDateTime(draft.validFrom),
    validUntil: fromLocalDateTime(draft.validUntil),
    trafficLimitBytes: draft.trafficLimitBytes,
    resetStrategy: draft.resetStrategy,
    squadUuids: [...draft.squadUuids],
    reason: draft.reason.trim(),
  })
}
</script>

<template>
  <USlideover :open="open" :title="t('adminUserProfile.manualGrant.title')" :description="t('adminUserProfile.manualGrant.hint')" :dismissible="!busy" :close="!busy" :ui="{ footer: 'justify-end' }" @update:open="!busy && emit('update:open', $event)">
    <template #body>
      <UAlert color="warning" variant="soft" icon="i-ph-warning-circle" :title="t('adminUserProfile.manualGrant.warning')" :description="t('adminUserProfile.manualGrant.warningHint')" />
      <AnimatePresence mode="wait" :initial="false">
        <motion.div :key="busy ? 'processing' : error ? `error:${error}` : 'draft'" layout :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
          <UForm id="manual-entitlement-form" :state="draft" class="form-stack" @submit="submit">
            <UFormField name="comboId" :label="t('adminUserProfile.combo')" required>
              <USelect v-model="draft.comboId" class="w-full" :items="comboItems" value-key="value" :loading="optionsLoading" :disabled="busy" />
            </UFormField>
            <div class="admin-form-grid">
              <UFormField name="validFrom" :label="t('adminUserProfile.validFrom')" required><UInput v-model="draft.validFrom" type="datetime-local" :disabled="busy" /></UFormField>
              <UFormField name="validUntil" :label="t('adminUserProfile.validUntil')" required><UInput v-model="draft.validUntil" type="datetime-local" :disabled="busy" /></UFormField>
            </div>
            <motion.div v-if="draft.validFrom && draft.validUntil && !datesValid" :initial="{ opacity: 0 }" :animate="{ opacity: 1 }" :transition="{ duration: reducedMotion ? 0.08 : 0.16, ease: 'easeOut' }"><UAlert color="error" variant="soft" :description="t('adminUserProfile.invalidDates')" /></motion.div>
            <div class="admin-form-grid">
              <UFormField name="trafficLimitBytes" :label="t('adminUserProfile.trafficLimit')" :error="trafficError" required><UInput v-model.trim="draft.trafficLimitBytes" inputmode="numeric" :disabled="busy" /></UFormField>
              <UFormField name="resetStrategy" :label="t('adminUserProfile.resetStrategy')" required><USelect v-model="draft.resetStrategy" :items="cadenceItems" value-key="value" :disabled="busy" /></UFormField>
            </div>
            <UFormField name="squadUuids" :label="t('adminUserProfile.squads')" :hint="t('adminUserProfile.manualGrant.squadsHint')">
              <USelectMenu v-model="draft.squadUuids" class="w-full" :items="squadItems" value-key="value" label-key="label" multiple :loading="optionsLoading" :disabled="busy" />
            </UFormField>
            <UFormField name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="draft.reason" :rows="3" :minlength="3" :maxlength="500" :disabled="busy" /></UFormField>
            <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
          </UForm>
        </motion.div>
      </AnimatePresence>
    </template>
    <template #footer>
      <UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="emit('update:open', false)" />
      <UButton type="submit" form="manual-entitlement-form" icon="i-ph-plus" :label="busy ? t('common.working') : t('adminUserProfile.manualGrant.submit')" :loading="busy" :disabled="!canSave || busy" />
    </template>
  </USlideover>
</template>
