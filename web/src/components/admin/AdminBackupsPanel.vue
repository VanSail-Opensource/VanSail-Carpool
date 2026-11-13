<script setup lang="ts">
import { computed, onScopeDispose, reactive, shallowRef } from 'vue'
import { api } from '@/api/client'
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
const deleting = shallowRef(false)
const downloading = shallowRef(false)
let disposed = false
const backupDeleteTarget = shallowRef<BackupRecord | null>(null)
const jobDeleteTarget = shallowRef<JobRecord | null>(null)
const removedBackups = reactive(new Set<string>())
const removedJobs = reactive(new Set<string>())
const visibleBackups = computed(() => backups.items.value.filter(backup => !removedBackups.has(backup.id)))
const visibleJobs = computed(() => jobs.items.value.filter(job => !removedJobs.has(job.id)))
const { t } = useI18n()
const workspace = shallowRef('backups')
const tabs = computed(() => [{ value: 'backups', label: t('adminBackups.title') }, { value: 'jobs', label: t('adminBackups.jobs') }, { value: 'tools', label: t('browser.tools') }])
function createBackup(): void {
  if (disposed || deleting.value || backups.busy.value) return
  void backups.perform(() => api.createAdminBackup())
}
function refreshSections(): void {
  if (disposed) return
  void Promise.allSettled([backups.load(), jobs.load()])
}
function retryJob(id: string): void {
  if (disposed || deleting.value || jobs.busy.value || removedJobs.has(id)) return
  void jobRetry.execute(id, `job-retry:${id}`, (key) => api.retryAdminJob(id, key))
}
function requestBackupDelete(backup: BackupRecord): void {
  if (disposed || deleting.value || downloading.value || backups.busy.value || removedBackups.has(backup.id)) return
  actionError.value = null
  backupDeleteTarget.value = backup
}
function requestJobDelete(job: JobRecord): void {
  if (disposed || deleting.value || jobs.busy.value || jobRetry.blocksMutations.value || removedJobs.has(job.id)) return
  actionError.value = null
  jobDeleteTarget.value = job
}
async function deleteBackup(): Promise<void> {
  const target = backupDeleteTarget.value
  if (disposed || !target || deleting.value || backups.busy.value) return
  deleting.value = true
  actionError.value = null
  try {
    await featuresApi.deleteBackup(target.id)
    if (disposed) return
    removedBackups.add(target.id)
    backupDeleteTarget.value = null
    await backups.load()
  } catch (caught) {
    if (!disposed) actionError.value = localizedError(caught, 'adminBackups.deleteFailed')
  } finally {
    if (!disposed) deleting.value = false
  }
}

async function deleteJob(): Promise<void> {
  const target = jobDeleteTarget.value
  if (disposed || !target || deleting.value || jobs.busy.value || jobRetry.blocksMutations.value) return
  deleting.value = true
  actionError.value = null
  try {
    await api.deleteAdminJob(target.id)
    if (disposed) return
    removedJobs.add(target.id)
    jobDeleteTarget.value = null
    await jobs.load()
  } catch (caught) {
    if (!disposed) actionError.value = localizedError(caught, 'adminBackups.jobDeleteFailed')
  } finally {
    if (!disposed) deleting.value = false
  }
}

function backupName(backup: BackupRecord): string {
  return backup.path.split(/[\\/]/).pop() || t('adminBackups.backupName', { id: backup.id })
}

async function download(backup: BackupRecord): Promise<void> {
  if (disposed || downloading.value || deleting.value || removedBackups.has(backup.id)) return
  downloading.value = true
  actionError.value = null
  try {
    const blob = await featuresApi.downloadBackup(backup.id)
    if (disposed) return
    const url = globalThis.URL.createObjectURL(blob)
    try {
      const anchor = globalThis.document.createElement('a')
      anchor.href = url
      anchor.download = backupName(backup)
      anchor.click()
    } finally {
      globalThis.URL.revokeObjectURL(url)
    }
  } catch (caught) {
    if (!disposed) actionError.value = localizedError(caught, 'adminBackups.downloadFailed')
  } finally {
    if (!disposed) downloading.value = false
  }
}
onScopeDispose(() => { disposed = true })
</script>
<template>
  <section class="admin-panel">
    <header class="admin-panel__heading"><div><h2>{{ t('adminBackups.title') }}</h2><p>{{ t('adminBackups.copy') }}</p></div><UButton icon="i-ph-archive" :disabled="backups.busy.value || deleting" :loading="backups.busy.value" :label="t('adminBackups.create')" @click="createBackup" /></header>
    <UTabs v-model="workspace" :items="tabs" :content="false" class="admin-workspace-tabs" />
    <InlineNotice v-if="actionError" tone="warning">{{ actionError }}</InlineNotice>
    <AdminSectionState v-show="workspace === 'backups'" :loading="backups.loading.value" :error="backups.error.value" @retry="backups.load()"><BackupInventory :items="visibleBackups" :busy="backups.busy.value || deleting || downloading" @download="download" @delete="requestBackupDelete" /></AdminSectionState>
    <div v-show="workspace === 'jobs'"><OperationStatusNotice :receipt="jobRetry.receipt.value" :error="jobRetry.error.value" :checking="jobRetry.checking.value" @refresh="jobRetry.refresh" /><AdminSectionState :loading="jobs.loading.value" :error="jobs.error.value" @retry="jobs.load()"><JobInventory :items="visibleJobs" :busy="jobs.busy.value || jobRetry.blocksMutations.value || deleting" :active-id="jobRetry.busy.value ? jobRetry.activeCommandId.value : null" @retry="retryJob" @delete="requestJobDelete" /><div v-if="jobs.nextCursor.value" class="admin-pagination"><UButton color="neutral" variant="outline" :loading="jobs.loading.value" :disabled="jobs.loading.value" :label="t('adminBackups.loadMoreJobs')" @click="jobs.loadMore" /></div></AdminSectionState></div>
    <div v-show="workspace === 'tools'" class="backup-tools"><MaintenanceTrigger @completed="refreshSections" /></div>
    <ConfirmDialog :open="Boolean(backupDeleteTarget)" :title="t('adminBackups.deleteTitle', { name: backupDeleteTarget ? backupName(backupDeleteTarget) : t('adminBackups.backup') })" :description="t('adminBackups.deleteDescription')" :confirm-label="t('adminBackups.deleteBackup')" :error="actionError" :busy="deleting" danger @update:open="!$event && !deleting && (backupDeleteTarget = null)" @confirm="deleteBackup" />
    <ConfirmDialog :open="Boolean(jobDeleteTarget)" :title="t('adminBackups.deleteJobTitle')" :description="t('adminBackups.deleteJobDescription')" :confirm-label="t('adminBackups.deleteJob')" :error="actionError" :busy="deleting" danger @update:open="!$event && !deleting && (jobDeleteTarget = null)" @confirm="deleteJob" />
  </section>
</template>
<style scoped>
.backup-tools { display: grid; gap: 24px; max-width: 800px; }
</style>
