<script setup lang="ts">
import { restoreItems } from '@/api/cache/restore'
import { computed, onMounted, reactive, shallowRef } from 'vue'

import type { QuestionnaireAdminRecord } from '@/api/features'
import { featuresApi } from '@/api/features'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { localizedError, useI18n } from '@/i18n'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'
import QuestionnaireImportWorkflow from './questionnaires/QuestionnaireImportWorkflow.vue'

const items = shallowRef<QuestionnaireAdminRecord[]>([])
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const editingId = shallowRef<string | null | undefined>(undefined)
const importing = shallowRef<QuestionnaireAdminRecord | null>(null)
const closing = shallowRef<QuestionnaireAdminRecord | null>(null)
const deleting = shallowRef<QuestionnaireAdminRecord | null>(null)
const draft = reactive({ title: '', description: '', formUrl: '', rewardTxb: '5.00' })
const { t } = useI18n()
const columns = computed(() => [{ accessorKey: 'title', header: t('adminQuestionnaires.formTitle') }, { id: 'status', header: t('adminWorkspace.status') }, { accessorKey: 'participantCount', header: t('adminWorkspace.participants') }, { id: 'reward', header: t('adminWorkspace.reward') }, { id: 'actions', header: t('adminWorkspace.actions') }])

async function load(): Promise<void> {
  loading.value = !restoreItems('/api/v1/admin/questionnaires', items); error.value = null
  try { items.value = (await featuresApi.getAdminQuestionnaires()).items }
  catch (caught) { error.value = localizedError(caught, 'adminQuestionnaires.loadFailed') }
  finally { loading.value = false }
}

function edit(questionnaire?: QuestionnaireAdminRecord): void {
  editingId.value = questionnaire?.id ?? null
  Object.assign(draft, questionnaire ? {
    title: questionnaire.title, description: questionnaire.description, formUrl: questionnaire.formUrl,
    rewardTxb: txbInputFromMinor(questionnaire.rewardTxbMinor),
  } : { title: '', description: '', formUrl: '', rewardTxb: '5.00' })
}

async function save(): Promise<void> {
  const rewardTxbMinor = moneyFromTxbInput(draft.rewardTxb)
  if (!rewardTxbMinor) return
  busy.value = true; error.value = null
  try { await featuresApi.saveAdminQuestionnaire(editingId.value ?? null, { title: draft.title, description: draft.description, formUrl: draft.formUrl, rewardTxbMinor }); editingId.value = undefined; await load() }
  catch (caught) { error.value = localizedError(caught, 'adminQuestionnaires.saveFailed') }
  finally { busy.value = false }
}

async function activate(id: string): Promise<void> {
  busy.value = true; error.value = null
  try { await featuresApi.activateAdminQuestionnaire(id); await load() }
  catch (caught) { error.value = localizedError(caught, 'adminQuestionnaires.activateFailed') }
  finally { busy.value = false }
}

async function closeQuestionnaire(): Promise<void> {
  if (!closing.value) return
  busy.value = true; error.value = null
  try { await featuresApi.closeAdminQuestionnaire(closing.value.id); closing.value = null; await load() }
  catch (caught) { error.value = localizedError(caught, 'adminQuestionnaires.closeFailed') }
  finally { busy.value = false }
}

async function remove(): Promise<void> {
  if (!deleting.value || busy.value) return
  busy.value = true; error.value = null
  try { await featuresApi.deleteAdminQuestionnaire(deleting.value.id); deleting.value = null; await load() }
  catch (caught) { error.value = localizedError(caught, 'adminQuestionnaires.deleteFailed') }
  finally { busy.value = false }
}

function statusLabel(status: QuestionnaireAdminRecord['status']): string {
  if (status === 'active') return t('common.active')
  return t(`adminQuestionnaires.status.${status}`)
}

onMounted(() => void load())
</script>

<template>
  <section class="admin-panel">
    <div class="admin-panel__heading"><div><h2>{{ t('adminQuestionnaires.title') }}</h2><p>{{ t('adminQuestionnaires.copy') }}</p></div><UButton icon="i-ph-plus" :label="t('adminQuestionnaires.new')" @click="edit()" /></div>
    <USlideover :open="editingId !== undefined" :title="editingId ? t('adminQuestionnaires.edit') : t('adminQuestionnaires.draft')" :description="t('adminQuestionnaires.copy')" :dismissible="!busy" @update:open="!$event && (editingId = undefined)">
      <template #body>
        <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        <form class="admin-editor-form" @submit.prevent="save">
          <div class="catalog-editor__heading"><h3>{{ editingId ? t('adminQuestionnaires.edit') : t('adminQuestionnaires.draft') }}</h3></div>
          <UFormField name="title" :label="t('adminQuestionnaires.formTitle')" required><UInput v-model.trim="draft.title" :maxlength="120" /></UFormField>
          <UFormField name="url" :label="t('adminQuestionnaires.formUrl')" required><UInput v-model.trim="draft.formUrl" type="url" /></UFormField>
          <UFormField class="catalog-editor__wide" name="description" :label="t('adminQuestionnaires.description')" required><UTextarea v-model.trim="draft.description" :rows="3" /></UFormField>
          <TxbAmountField id="questionnaire-reward" v-model="draft.rewardTxb" :label="t('adminQuestionnaires.reward')" min-minor="1" required />
          <div class="button-row"><UButton color="neutral" variant="outline" :label="t('common.cancel')" @click="editingId = undefined" /><UButton type="submit" :disabled="busy" :loading="busy" :label="busy ? t('common.saving') : t('adminQuestionnaires.saveDraft')" /></div>
        </form>
      </template>
    </USlideover>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <UTable class="admin-table" :data="items" :columns="columns" :loading="loading">
      <template #loading><div class="admin-loading" role="status"><USkeleton class="h-10 w-full" /><span>{{ t('common.loading') }}</span></div></template>
      <template #status-cell="{ row }"><UBadge :color="row.original.status === 'active' ? 'success' : 'neutral'" variant="subtle" :label="statusLabel(row.original.status)" /></template>
      <template #reward-cell="{ row }">{{ txbInputFromMinor(row.original.rewardTxbMinor) }} TXB</template>
      <template #actions-cell="{ row }">
        <div class="admin-row-actions">
          <UButton v-if="row.original.status === 'draft'" color="neutral" variant="ghost" :disabled="busy" :label="t('adminQuestionnaires.activate')" @click="activate(row.original.id)" />
          <UButton v-if="row.original.status === 'active'" color="neutral" variant="ghost" :disabled="busy" :label="t('common.close')" @click="closing = row.original" />
          <UButton v-if="['active', 'closed', 'settled'].includes(row.original.status)" color="neutral" variant="ghost" icon="i-ph-file-csv" :label="t('adminQuestionnaires.importCsv')" @click="importing = row.original" />
          <UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminQuestionnaires.editNamed', { name: row.original.title })" @click="edit(row.original)" />
          <UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminQuestionnaires.deleteNamed', { name: row.original.title })" @click="deleting = row.original" />
        </div>
      </template>
      <template #empty>{{ t('adminQuestionnaires.none') }}</template>
    </UTable>
    <QuestionnaireImportWorkflow v-if="importing" :questionnaire="importing" @close="importing = null" />
    <ConfirmDialog :open="Boolean(closing)" :title="t('adminQuestionnaires.closeTitle', { name: closing?.title ?? t('adminQuestionnaires.questionnaire') })" :description="t('adminQuestionnaires.closeDescription')" :confirm-label="t('common.close')" :busy="busy" @update:open="!$event && (closing = null)" @confirm="closeQuestionnaire" />
    <ConfirmDialog :open="Boolean(deleting)" :title="t('adminQuestionnaires.deleteTitle', { name: deleting?.title ?? t('adminQuestionnaires.questionnaire') })" :description="t('adminQuestionnaires.deleteDescription')" :confirm-label="t('adminQuestionnaires.deletePermanently')" :busy="busy" danger @update:open="!$event && (deleting = null)" @confirm="remove" />
  </section>
</template>

<style scoped>
.questionnaire-row { align-items: center; }
.row-actions { display: flex; align-items: center; flex-wrap: wrap; justify-content: flex-end; gap: 0.4rem; }
</style>
