<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { BackupRecord, JobRecord } from '@/api/types'
import { featuresApi } from '@/api/features'
import InlineNotice from '@/components/common/InlineNotice.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import { useAdminSection } from '@/composables/useAdminSection'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { localizedError, useI18n } from '@/i18n'
import AdminSectionState from './AdminSectionState.vue'
import MaintenanceTrigger from './backups/MaintenanceTrigger.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BackupInventory from './backups/BackupInventory.vue'
import JobInventory from './backups/JobInventory.vue'
const backups = useAdminSection<BackupRecord>('backups')
const jobs = useAdminSection<JobRecord>('jobs')
const jobRetry = useDurableCommand({ errorKey: 'errors.adminAction', onTerminal: (receipt) => receipt.status === 'succeeded' ? jobs.load() : undefined })
const actionError = shallowRef<string | null>(null)
const backupDeleteTarget = shallowRef<BackupRecord | null>(null)
const jobDeleteTarget = shallowRef<JobRecord | null>(null)
const { t } = useI18n()
const workspace = shallowRef('backups')
const tabs = computed(() => [{ value: 'backups', label: t('adminBackups.title') }, { value: 'jobs', label: t('adminBackups.jobs') }, { value: 'tools', label: t('browser.tools') }])
function createBackup(): void {
  void backups.perform(() => import('@/api/client').then(({ api }) => api.createAdminBackup()))
}
function refreshSections(): void {
  void Promise.allSettled([backups.load(), jobs.load()])
}
function retryJob(id: string): void {
  void jobRetry.execute(id, `job-retry:${id}`, (key) => import('@/api/client').then(({ api }) => api.retryAdminJob(id, key)))
}
async function deleteBackup(): Promise<void> {
  if (!backupDeleteTarget.value) return
  actionError.value = null
  try {
    await featuresApi.deleteBackup(backupDeleteTarget.value.id)
    backupDeleteTarget.value = null
    await backups.load()
  } catch (caught) {
    actionError.value = localizedError(caught, 'adminBackups.deleteFailed')
  }
}

async function deleteJob(): Promise<void> {
  if (!jobDeleteTarget.value) return
  actionError.value = null
  try {
    const { api } = await import('@/api/client')
    await api.deleteAdminJob(jobDeleteTarget.value.id)
    jobDeleteTarget.value = null
    await jobs.load()
  } catch (caught) {
    actionError.value = localizedError(caught, 'adminBackups.jobDeleteFailed')
  }
}

function backupName(backup: BackupRecord): string {
  return backup.path.split(/[\\/]/).pop() ?? t('adminBackups.backupName', { id: backup.id })
}

async function download(backup: BackupRecord): Promise<void> {
  actionError.value = null
  try {
    const blob = await featuresApi.downloadBackup(backup.id)
    const url = globalThis.URL.createObjectURL(blob)
    const anchor = globalThis.document.createElement('a')
    anchor.href = url
    anchor.download = backupName(backup)
    anchor.click()
    globalThis.URL.revokeObjectURL(url)
  } catch (caught) {
    actionError.value = localizedError(caught, 'adminBackups.downloadFailed')
  }
}

</script>
<template>
  <section class="admin-panel">
    <header class="admin-panel__heading"><div><h2>{{ t('adminBackups.title') }}</h2><p>{{ t('adminBackups.copy') }}</p></div><UButton icon="i-ph-archive" :disabled="backups.busy.value" :loading="backups.busy.value" :label="t('adminBackups.create')" @click="createBackup" /></header>
    <UTabs v-model="workspace" :items="tabs" :content="false" class="admin-workspace-tabs" />
    <InlineNotice v-if="actionError" tone="warning">{{ actionError }}</InlineNotice>
    <AdminSectionState v-show="workspace === 'backups'" :loading="backups.loading.value" :error="backups.error.value" @retry="backups.load()"><BackupInventory :items="backups.items.value" :busy="backups.busy.value" @download="download" @delete="backupDeleteTarget = $event" /></AdminSectionState>
    <div v-show="workspace === 'jobs'"><OperationStatusNotice :receipt="jobRetry.receipt.value" :error="jobRetry.error.value" :checking="jobRetry.checking.value" @refresh="jobRetry.refresh" /><AdminSectionState :loading="jobs.loading.value" :error="jobs.error.value" @retry="jobs.load()"><JobInventory :items="jobs.items.value" :busy="jobs.busy.value || jobRetry.blocksMutations.value" :active-id="jobRetry.busy.value ? jobRetry.activeCommandId.value : null" @retry="retryJob" @delete="jobDeleteTarget = $event" /><div v-if="jobs.nextCursor.value" class="admin-pagination"><UButton color="neutral" variant="outline" :loading="jobs.loading.value" :disabled="jobs.loading.value" :label="t('adminBackups.loadMoreJobs')" @click="jobs.loadMore" /></div></AdminSectionState></div>
    <div v-show="workspace === 'tools'" class="backup-tools"><MaintenanceTrigger @completed="refreshSections" /></div>
    <ConfirmDialog :open="Boolean(backupDeleteTarget)" :title="t('adminBackups.deleteTitle', { name: backupDeleteTarget ? backupName(backupDeleteTarget) : t('adminBackups.backup') })" :description="t('adminBackups.deleteDescription')" :confirm-label="t('adminBackups.deleteBackup')" :error="actionError" danger @update:open="!$event && (backupDeleteTarget = null)" @confirm="deleteBackup" />
    <ConfirmDialog :open="Boolean(jobDeleteTarget)" :title="t('adminBackups.deleteJobTitle')" :description="t('adminBackups.deleteJobDescription')" :confirm-label="t('adminBackups.deleteJob')" :error="actionError" danger @update:open="!$event && (jobDeleteTarget = null)" @confirm="deleteJob" />
  </section>
</template>
<style scoped>
.backup-tools { display: grid; gap: 24px; max-width: 800px; }
</style>
