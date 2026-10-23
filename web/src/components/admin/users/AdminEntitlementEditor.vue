<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { motion } from 'motion-v'

import type { AdminEntitlement, Combo, EntitlementEditRequest, SquadProduct } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { fromLocalDateTime, toLocalDateTime } from './adminUserFormat'

interface EditorDraft {
  comboId: string
  validFrom: string
  validUntil: string
  status: AdminEntitlement['status']
  trafficLimitBytes: string
  resetStrategy: AdminEntitlement['resetStrategy']
  squadUuids: string[]
  reason: string
}

const props = defineProps<{
  open: boolean
  item: AdminEntitlement | null
  combos: Combo[]
  squads: SquadProduct[]
  busy: boolean
  optionsLoading: boolean
  error?: string | null
}>()
const emit = defineEmits<{ 'update:open': [value: boolean]; save: [body: EntitlementEditRequest] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const draft = reactive<EditorDraft>({
  comboId: '', validFrom: '', validUntil: '', status: 'active', trafficLimitBytes: '',
  resetStrategy: 'MONTH_ROLLING', squadUuids: [], reason: '',
})

const comboItems = computed(() => props.combos.map((combo) => ({ label: combo.name, value: combo.id })))
const squadItems = computed(() => props.squads.map((squad) => ({ label: squad.name, value: squad.groupId })))
const statusItems = computed(() => ['activating', 'active', 'queued', 'expired', 'cancelled', 'failed']
  .map((value) => ({ value, label: t(`adminUserProfile.entitlementStatus.${value}`) })))
const cadenceItems = computed(() => ['DAY', 'WEEK', 'MONTH_ROLLING']
  .map((value) => ({ value, label: t(`adminUserProfile.cadence.${value}`) })))
const datesValid = computed(() => {
  const from = new Date(draft.validFrom).getTime()
  const until = new Date(draft.validUntil).getTime()
  return Number.isFinite(from) && Number.isFinite(until) && until > from
})
const canSave = computed(() => Boolean(props.item && draft.comboId && datesValid.value &&
  /^[1-9]\d*$/.test(draft.trafficLimitBytes) && draft.reason.trim().length >= 3))

watch(() => props.item, (item) => {
  if (!item) return
  Object.assign(draft, {
    comboId: item.comboId,
    validFrom: toLocalDateTime(item.validFrom),
    validUntil: toLocalDateTime(item.validUntil),
    status: item.status,
    trafficLimitBytes: item.trafficLimitBytes,
    resetStrategy: item.resetStrategy,
    squadUuids: [...item.squadUuids],
    reason: '',
  })
}, { immediate: true })

function submit(): void {
  if (!props.item || !canSave.value) return
  emit('save', {
    version: props.item.updatedAt,
    reason: draft.reason.trim(),
    comboId: draft.comboId,
    validFrom: fromLocalDateTime(draft.validFrom),
    validUntil: fromLocalDateTime(draft.validUntil),
    status: draft.status,
    trafficLimitBytes: draft.trafficLimitBytes,
    resetStrategy: draft.resetStrategy,
    squadUuids: [...draft.squadUuids],
  })
}
</script>

<template>
  <USlideover :open="open" :title="t('adminUserProfile.editorTitle')" :description="t('adminUserProfile.editorHint')" :dismissible="!busy" :ui="{ footer: 'justify-end' }" @update:open="emit('update:open', $event)">
    <template #body>
      <UAlert color="warning" variant="soft" icon="i-ph-warning-circle" :title="t('adminUserProfile.editorWarning')" :description="t('adminUserProfile.editorWarningHint')" />
      <motion.div layout :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
        <UForm id="entitlement-editor" :state="draft" class="form-stack" @submit="submit">
          <UFormField name="comboId" :label="t('adminUserProfile.combo')" required>
            <USelect v-model="draft.comboId" class="w-full" :items="comboItems" value-key="value" :loading="optionsLoading" />
          </UFormField>
          <div class="admin-form-grid">
            <UFormField name="validFrom" :label="t('adminUserProfile.validFrom')" required><UInput v-model="draft.validFrom" type="datetime-local" /></UFormField>
            <UFormField name="validUntil" :label="t('adminUserProfile.validUntil')" required><UInput v-model="draft.validUntil" type="datetime-local" /></UFormField>
          </div>
          <motion.div v-if="draft.validFrom && draft.validUntil && !datesValid" :initial="{ opacity: 0 }" :animate="{ opacity: 1 }" :transition="{ duration: reducedMotion ? 0.08 : 0.16, ease: 'easeOut' }"><UAlert color="error" variant="soft" :description="t('adminUserProfile.invalidDates')" /></motion.div>
          <div class="admin-form-grid">
            <UFormField name="status" :label="t('adminUserProfile.status')" required><USelect v-model="draft.status" :items="statusItems" value-key="value" /></UFormField>
            <UFormField name="resetStrategy" :label="t('adminUserProfile.resetStrategy')" required><USelect v-model="draft.resetStrategy" :items="cadenceItems" value-key="value" /></UFormField>
          </div>
          <UFormField name="trafficLimitBytes" :label="t('adminUserProfile.trafficLimit')" required><UInput v-model.trim="draft.trafficLimitBytes" inputmode="numeric" /></UFormField>
          <UFormField name="squadUuids" :label="t('adminUserProfile.squads')"><USelectMenu v-model="draft.squadUuids" class="w-full" :items="squadItems" value-key="value" label-key="label" multiple :loading="optionsLoading" /></UFormField>
          <UFormField name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="draft.reason" :rows="3" :minlength="3" :maxlength="500" /></UFormField>
          <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        </UForm>
      </motion.div>
    </template>
    <template #footer>
      <UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="emit('update:open', false)" />
      <UButton type="submit" form="entitlement-editor" icon="i-ph-floppy-disk" :label="busy ? t('common.working') : t('common.save')" :loading="busy" :disabled="!canSave || busy" />
    </template>
  </USlideover>
</template>
