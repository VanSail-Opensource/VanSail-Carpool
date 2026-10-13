<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { DatabaseMutationReview, DatabaseRow, DatabaseTable, DatabaseValue } from '@/api/features'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import DatabaseMutationReviewPanel from './DatabaseMutationReviewPanel.vue'
import DatabaseRecordFields from './DatabaseRecordFields.vue'
import type { DeepReadonly } from './types'

const props = defineProps<{
  table: DatabaseTable
  action: 'insert' | 'update' | 'delete'
  row?: DatabaseRow
  review: DeepReadonly<DatabaseMutationReview> | null
  busy: boolean
  error?: string | null
}>()
const emit = defineEmits<{
  cancel: []
  invalidate: []
  review: [payload: { action: 'insert' | 'update' | 'delete'; row?: DatabaseRow; values: Record<string, DatabaseValue>; reason: string }]
  apply: [payload: { action: 'insert' | 'update' | 'delete'; row?: DatabaseRow; values: Record<string, DatabaseValue>; reason: string; confirmation: string }]
}>()
const draft = reactive<Record<string, DatabaseValue>>({})
const textDraft = reactive<Record<string, string>>({})
const nullDraft = reactive<Record<string, boolean>>({})
const reason = shallowRef('')
const confirmation = shallowRef('')
const formId = 'database-record-form'
const { t } = useI18n()
const canReview = computed(() => reason.value.trim().length >= 4)
const canApply = computed(() => Boolean(props.review && confirmation.value === props.review.requiredConfirmation))
const title = computed(() => t('databaseRecord.title', { action: t(`databaseRecord.actions.${props.action}`), table: props.table.name }))
const drawerUi = {
  content: 'database-record-editor__content',
  header: 'database-record-editor__header',
  body: 'database-record-editor__scroll-body',
  footer: 'database-record-editor__footer',
} as const
const workflowState = computed(() => props.busy ? 'processing' : props.review ? 'review' : props.action)
const { reducedMotion } = useMotionPreferences()

watch([() => props.row, () => props.action, () => props.table], ([row]) => {
  for (const key of Object.keys(draft)) delete draft[key]
  for (const key of Object.keys(textDraft)) delete textDraft[key]
  for (const key of Object.keys(nullDraft)) delete nullDraft[key]
  for (const column of props.table.columns) {
    const value = row?.values[column.name] ?? (column.nullable ? null : /BOOL/i.test(column.declaredType) ? false : '')
    draft[column.name] = value
    nullDraft[column.name] = value === null
    textDraft[column.name] = typeof value === 'object' && value !== null ? value.blobBase64 : value === null ? '' : String(value)
  }
  reason.value = ''
  confirmation.value = ''
}, { immediate: true })

watch(() => props.review, (review) => { if (!review) confirmation.value = '' })

function updateText(column: string, value: string): void { textDraft[column] = value; emit('invalidate') }
function updateBoolean(column: string, value: boolean): void { draft[column] = value; emit('invalidate') }
function updateNull(column: string, value: boolean): void { nullDraft[column] = value; emit('invalidate') }

function values(): Record<string, DatabaseValue> {
  const output: Record<string, DatabaseValue> = {}
  for (const column of props.table.columns) {
    const current = draft[column.name]
    if (!column.editable || column.sensitive) continue
    if (nullDraft[column.name]) output[column.name] = null
    else if ((typeof current === 'object' && current !== null) || /BLOB|BINARY/i.test(column.declaredType)) output[column.name] = { blobBase64: textDraft[column.name] ?? '' }
    else output[column.name] = typeof current === 'boolean' ? current : textDraft[column.name]
  }
  return output
}

function submit(): void {
  const payload = { action: props.action, row: props.row, values: values(), reason: reason.value.trim() }
  if (!props.review && canReview.value) emit('review', payload)
  else if (props.review && canApply.value) emit('apply', { ...payload, confirmation: confirmation.value })
}
</script>

<template>
  <USlideover
    class="database-record-editor-drawer"
    :open="true"
    :title="title"
    :description="t('databaseRecord.copy')"
    :dismissible="!busy"
    :ui="drawerUi"
    @update:open="!$event && emit('cancel')"
  >
    <template #close>
      <UButton icon="i-ph-x" color="neutral" variant="ghost" :aria-label="t('databaseRecord.close')" />
    </template>
    <template #body>
      <UAlert v-if="error" color="warning" variant="soft" :title="error" />
      <div class="database-record-editor__body">
        <AnimatePresence mode="wait" :initial="false">
          <motion.div :key="workflowState" class="database-record-editor__workflow" layout :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
            <UAlert v-if="!review" color="warning" variant="soft" icon="i-ph-warning" :description="table.warning || t('databaseRecord.warning')" />
            <form :id="formId" class="database-form" @submit.prevent="submit">
              <DatabaseMutationReviewPanel v-if="review" v-model="confirmation" :review="review" />
              <template v-else>
                <div v-if="action === 'delete'" class="database-delete-summary">
                  <strong>{{ t('databaseRecord.recordKey') }}</strong>
                  <code>{{ JSON.stringify(row?.key ?? {}) }}</code>
                  <p>{{ t('databaseRecord.deleteHint') }}</p>
                </div>
                <DatabaseRecordFields
                  v-else
                  :columns="table.columns"
                  :draft="draft"
                  :text-draft="textDraft"
                  :null-draft="nullDraft"
                  @update:text="updateText"
                  @update:boolean="updateBoolean"
                  @update:null="updateNull"
                />
                <UFormField name="reason" :label="t('databaseRecord.reason')" required>
                  <UTextarea
                    v-model.trim="reason"
                    class="database-record-editor__reason"
                    :rows="3"
                    :minlength="4"
                    :placeholder="t('databaseRecord.reasonPlaceholder')"
                    @update:model-value="emit('invalidate')"
                  />
                </UFormField>
              </template>
            </form>
          </motion.div>
        </AnimatePresence>
      </div>
    </template>
    <template #footer>
      <div class="database-form-actions button-row">
        <UButton v-if="review" type="button" color="neutral" variant="outline" :disabled="busy" :label="t('databaseRecord.changeDraft')" @click="emit('invalidate')" />
        <UButton type="submit" :form="formId" icon="i-ph-floppy-disk" :loading="busy" :disabled="busy || (review ? !canApply : !canReview)" :label="busy ? t('databaseRecord.working') : review ? t('databaseRecord.apply') : t('databaseRecord.reviewChange')" />
      </div>
    </template>
  </USlideover>
</template>

<style scoped>
.database-record-editor__body,
.database-record-editor__workflow,
.database-form { display: grid; min-width: 0; gap: 0.8rem; }
.database-record-editor__reason :deep(textarea) { font-size: 1rem; }
.database-delete-summary { display: grid; gap: 0.45rem; padding: 0.8rem; border: 1px solid var(--danger); border-radius: var(--radius-control); background: var(--danger-soft); }
.database-delete-summary strong { color: var(--danger); font-size: 0.78rem; }
.database-delete-summary code { font-family: var(--font-mono); font-size: 0.68rem; overflow-wrap: anywhere; }
.database-delete-summary p { margin: 0; color: var(--text-muted); font-size: 0.68rem; }
.database-form-actions { justify-content: flex-end; }
:global(.database-record-editor__content) { width: min(100vw, 720px); max-width: min(100vw, 720px); }
:global(.database-record-editor__header) { flex: 0 0 auto; }
:global(.database-record-editor__scroll-body) { min-height: 0; overflow-y: auto; overscroll-behavior: contain; }
:global(.database-record-editor__footer) { flex: 0 0 auto; }


@media (max-width: 639px) {
  .database-form-actions { flex-direction: column; }
  .database-form-actions :deep(button) { width: 100%; }
}
</style>
