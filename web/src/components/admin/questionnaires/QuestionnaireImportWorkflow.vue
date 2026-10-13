<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { QuestionnaireAdminRecord } from '@/api/features'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useQuestionnaireImport } from '@/composables/useQuestionnaireImport'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { txbInputFromMinor } from '@/utils/format'

const props = defineProps<{ questionnaire: QuestionnaireAdminRecord }>()
const emit = defineEmits<{ close: [] }>()
const file = shallowRef<globalThis.File | null>(null)
const { preview, summary, codeColumn, busy, settlementImportId, operationReceipt, operationChecking, report, error, upload, analyze, settle, refreshOperation, reset } = useQuestionnaireImport(() => props.questionnaire.id)
const canSettle = computed(() => Boolean(summary.value && summary.value.matchedCount > 0 && !settlementImportId.value))
const { t } = useI18n()
const columnItems = computed(() => preview.value?.headers ?? [])
const tableData = computed(() => (preview.value?.sampleRows ?? []).map((row) => Object.fromEntries((preview.value?.headers ?? []).map((header, index) => [header, row[index] ?? '']))))
const tableColumns = computed(() => (preview.value?.headers ?? []).map((header) => ({ accessorKey: header, header })))
const workflowState = computed(() => report.value ? 'complete' : settlementImportId.value ? 'settling' : preview.value ? 'preview' : 'upload')
const { reducedMotion } = useMotionPreferences()

watch(() => props.questionnaire.id, () => { file.value = null; reset() })
watch(file, (next) => { if (next) void upload(next) })

function startOver(): void {
  file.value = null
  reset()
}
</script>

<template>
  <USlideover :open="true" :title="t('questionnaireImport.settle', { name: questionnaire.title })" :description="t('questionnaireImport.copy')" @update:open="!$event && emit('close')">
    <template #body>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <UButton v-if="operationReceipt && ['queued', 'processing'].includes(operationReceipt.status) && error" color="neutral" variant="outline" icon="i-ph-arrow-clockwise" :loading="operationChecking" :label="t('operations.checkStatus')" @click="refreshOperation" />
      <InlineNotice v-if="settlementImportId && preview?.status !== 'settled'" :tone="preview?.status === 'failed' ? 'warning' : 'success'" :title="preview?.status === 'processing' ? t('questionnaireImport.running') : preview?.status === 'failed' ? t('questionnaireImport.failed') : t('questionnaireImport.queued')">{{ t('questionnaireImport.durable', { id: settlementImportId }) }}</InlineNotice>
      <InlineNotice v-if="report" tone="success" :title="t('questionnaireImport.complete')">{{ t('questionnaireImport.completeCopy', { count: report.rewardedCount, reward: txbInputFromMinor(report.rewardTxbMinor) }) }}</InlineNotice>
      <AnimatePresence mode="wait" :initial="false">
        <motion.div
          :key="workflowState"
          layout
          :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 5 }"
          :animate="{ opacity: 1, y: 0 }"
          :exit="{ opacity: 0 }"
          :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }"
        >
          <div v-if="!preview" class="csv-drop">
            <UIcon name="i-ph-file-csv" class="text-4xl" />
            <h4>{{ t('questionnaireImport.csv') }}</h4><p>{{ t('questionnaireImport.csvHint') }}</p>
            <UFileUpload v-model="file" accept=".csv,text/csv" :label="busy ? t('questionnaireImport.uploading') : t('questionnaireImport.choose')" />
          </div>
          <template v-else>
            <div class="csv-meta"><UBadge color="neutral" variant="soft" :label="t('questionnaireImport.rows', { count: preview.dataRowCount })" /><UBadge color="neutral" variant="soft" :label="t('questionnaireImport.columns', { count: preview.headers.length })" /><UBadge color="neutral" variant="soft" :label="t('questionnaireImport.delimiter', { value: preview.delimiter })" /></div>
            <UFormField :label="t('questionnaireImport.codeColumn')"><USelect v-model="codeColumn" :items="columnItems" /></UFormField>
            <UTable :data="tableData" :columns="tableColumns" />
            <div v-if="summary" class="csv-summary"><UBadge color="success" variant="soft" :label="`${summary.matchedCount} ${t('questionnaireImport.matched')}`" /><UBadge color="neutral" variant="soft" :label="`${summary.duplicateCount} ${t('questionnaireImport.duplicate')}`" /><UBadge color="warning" variant="soft" :label="`${summary.unknownCount} ${t('questionnaireImport.unknown')}`" /><UBadge color="error" variant="soft" :label="`${summary.malformedCount} ${t('questionnaireImport.malformed')}`" /><UBadge color="neutral" variant="soft" :label="`${summary.alreadyAwardedCount} ${t('questionnaireImport.rewarded')}`" /></div>
            <div class="button-row"><UButton color="neutral" variant="outline" :disabled="busy" :label="t('questionnaireImport.startOver')" @click="startOver" /><UButton v-if="!summary" :disabled="busy || !codeColumn" :loading="busy" :label="busy ? t('questionnaireImport.analyzing') : t('questionnaireImport.analyze')" @click="analyze" /><UButton v-else icon="i-ph-check-circle" :disabled="busy || !canSettle" :loading="busy" :label="busy ? t('questionnaireImport.queueing') : t('questionnaireImport.confirm')" @click="settle" /></div>
          </template>
        </motion.div>
      </AnimatePresence>
    </template>
  </USlideover>
</template>

<style scoped>
.csv-drop { display: grid; place-items: center; min-height: 260px; padding: 1.2rem; border: 1px dashed var(--line-strong); border-radius: var(--radius-panel); color: var(--accent); text-align: center; }
.csv-drop h4, .csv-drop p { margin: 0; }
.csv-drop p { max-width: 34ch; margin: 0.4rem 0 1rem; color: var(--text-muted); font-size: 0.74rem; line-height: 1.45; }
.csv-meta, .csv-summary { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-block: 0.8rem; }
</style>
