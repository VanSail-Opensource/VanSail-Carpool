<script setup lang="ts">
import { computed, onScopeDispose, reactive, shallowRef, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import { adminOperationsApi, type AdminCatalogOptions, type BulkExtensionPreview, type OperationReceipt } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { localizedError, useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { createLatestRequest } from '@/utils/latestRequest'
import AdminDurationField from './AdminDurationField.vue'
import { durationMinutes, validDurationDraft } from './duration'

const open = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{ queued: [receipt: OperationReceipt] }>()
const { t } = useI18n()
const options = shallowRef<AdminCatalogOptions>({ combos: [], squads: [] })
const preview = shallowRef<BulkExtensionPreview | null>(null)
const optionsLoading = shallowRef(false)
const previewing = shallowRef(false)
const creating = shallowRef(false)
const error = shallowRef<string | null>(null)
const draft = reactive({
  comboIds: [] as string[], addonSquadUuids: [] as string[],
  duration: { amount: 1, unit: 'days' as const }, reason: '',
})
const attempts = new Map<string, string>()
const previews = createLatestRequest()
const mutations = createLatestRequest()
let disposed = false

const comboItems = computed(() => options.value.combos.map((combo) => ({ value: combo.id, label: combo.name })))
const squadItems = computed(() => options.value.squads.map((squad) => ({ value: squad.groupId, label: squad.name })))
const hasFilter = computed(() => draft.comboIds.length > 0 || draft.addonSquadUuids.length > 0)
const normalizedMinutes = computed(() => durationMinutes(draft.duration))
const canPreview = computed(() => hasFilter.value && validDurationDraft(draft.duration))
const canCreate = computed(() => canPreview.value && !previewing.value && preview.value !== null && preview.value.matchedUsers > 0 && draft.reason.trim().length >= 3)
const workflowState = computed(() => creating.value ? 'processing' : error.value ? `error:${error.value}` : preview.value ? 'review' : 'configure')
const { reducedMotion } = useMotionPreferences()

watch(() => [draft.comboIds.join(','), draft.addonSquadUuids.join(','), normalizedMinutes.value], () => {
  previews.invalidate()
  previewing.value = false
  preview.value = null
})

watch(open, (value) => {
  previews.invalidate()
  mutations.invalidate()
  previewing.value = false
  preview.value = null
  if (!value) return
  Object.assign(draft, { comboIds: [], addonSquadUuids: [], duration: { amount: 1, unit: 'days' }, reason: '' })
  preview.value = null
  error.value = null
  void loadOptions()
})

async function loadOptions(): Promise<void> {
  if (optionsLoading.value || options.value.combos.length) return
  optionsLoading.value = true
  try {
    const response = await adminOperationsApi.getCatalogOptions()
    if (!disposed) options.value = response
  } catch (caught) {
    if (!disposed && open.value) error.value = localizedError(caught, 'adminBulkExtension.optionsFailed')
  } finally {
    optionsLoading.value = false
  }
}

async function requestPreview(): Promise<void> {
  if (!canPreview.value || previewing.value || creating.value) return
  const ticket = previews.begin()
  previewing.value = true
  error.value = null
  try {
    const response = await adminOperationsApi.previewBulkExtension({
      comboIds: [...draft.comboIds], addonSquadUuids: [...draft.addonSquadUuids], durationMinutes: normalizedMinutes.value,
    })
    if (previews.isCurrent(ticket)) preview.value = response
  } catch (caught) {
    if (previews.isCurrent(ticket)) error.value = localizedError(caught, 'adminBulkExtension.previewFailed')
  } finally {
    if (previews.isCurrent(ticket)) previewing.value = false
  }
}

async function create(): Promise<void> {
  if (!canCreate.value || creating.value) return
  const body = {
    comboIds: [...draft.comboIds].sort(), addonSquadUuids: [...draft.addonSquadUuids].sort(),
    durationMinutes: normalizedMinutes.value, reason: draft.reason.trim(),
  }
  const fingerprint = JSON.stringify(body)
  const key = attempts.get(fingerprint) ?? createUuid()
  attempts.set(fingerprint, key)
  const ticket = mutations.begin()
  creating.value = true
  error.value = null
  try {
    const receipt = await adminOperationsApi.createBulkExtension(body, key)
    attempts.delete(fingerprint)
    if (!mutations.isCurrent(ticket)) return
    emit('queued', receipt)
    open.value = false
  } catch (caught) {
    if (mutations.isCurrent(ticket)) error.value = localizedError(caught, 'adminBulkExtension.createFailed')
  } finally {
    if (!disposed) creating.value = false
  }
}

onScopeDispose(() => { disposed = true; previews.dispose(); mutations.dispose(); attempts.clear() })

</script>

<template>
  <UModal v-model:open="open" :title="t('adminBulkExtension.title')" :description="t('adminBulkExtension.copy')" :dismissible="!creating" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }">
    <template #body>
      <AnimatePresence mode="wait" :initial="false">
        <motion.div :key="workflowState" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
          <UAlert color="info" variant="soft" icon="i-ph-funnel" :description="t('adminBulkExtension.orHint')" />
          <UForm :state="draft" class="form-stack" @submit="create">
            <fieldset class="form-stack border-0 p-0 m-0 min-w-0" :disabled="creating">
              <UFormField name="comboIds" :label="t('adminBulkExtension.combos')"><USelectMenu v-model="draft.comboIds" class="w-full" :items="comboItems" value-key="value" label-key="label" multiple :loading="optionsLoading" /></UFormField>
              <UFormField name="addonSquadUuids" :label="t('adminBulkExtension.addons')"><USelectMenu v-model="draft.addonSquadUuids" class="w-full" :items="squadItems" value-key="value" label-key="label" multiple :loading="optionsLoading" /></UFormField>
              <AdminDurationField v-model="draft.duration" />
              <UButton color="neutral" variant="outline" icon="i-ph-magnifying-glass" :label="previewing ? t('adminBulkExtension.previewing') : t('adminBulkExtension.preview')" :loading="previewing" :disabled="!canPreview || previewing || creating" @click="requestPreview" />
              <div v-if="preview" class="admin-preview-band">
                <div><strong>{{ preview.matchedUsers }}</strong><span>{{ t('adminBulkExtension.members') }}</span></div>
                <div><strong>{{ preview.activePurchases }}</strong><span>{{ t('adminBulkExtension.active') }}</span></div>
                <div><strong>{{ preview.queuedSuccessors }}</strong><span>{{ t('adminBulkExtension.queued') }}</span></div>
              </div>
              <UFormField v-if="preview" name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="draft.reason" :rows="3" :minlength="3" :maxlength="500" /></UFormField>
              <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
            </fieldset>
          </UForm>
        </motion.div>
      </AnimatePresence>
    </template>
    <template #footer>
      <UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="creating" @click="open = false" />
      <UButton color="warning" icon="i-ph-calendar-plus" :label="creating ? t('common.working') : t('adminBulkExtension.queue')" :loading="creating" :disabled="!canCreate || creating" @click="create" />
    </template>
  </UModal>
</template>
