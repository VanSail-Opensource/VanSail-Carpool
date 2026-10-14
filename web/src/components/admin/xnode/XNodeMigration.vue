<script setup lang="ts">
import { computed, reactive, shallowRef } from 'vue'
import type { TableColumn } from '@nuxt/ui'
import { z } from 'zod'
import { xnodeApi, type XNode, type XNodeMigration } from '@/api/xnode'
import { useI18n } from '@/i18n'
import AdminSectionState from '@/components/admin/AdminSectionState.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import XNodeJsonEditor from './XNodeJsonEditor.vue'
import { useXNodeMigration } from './useXNodeMigration'

const props = defineProps<{ nodes: XNode[] }>()
const { t } = useI18n()
const migration = useXNodeMigration()
const draft = reactive({ url: '', token: '', sourceBackupReference: '' })
const monitoring = shallowRef<unknown>([])
const jsonValid = shallowRef(true)
const inputSchema = z.object({ source: z.object({ url: z.url().regex(/^https?:\/\//), token: z.string().min(1) }), sourceBackupReference: z.string().trim().min(1).max(512), monitoring: z.array(z.object({ legacyUserId: z.string().regex(/^[1-9][0-9]*$/), serviceName: z.string().regex(/^[A-Za-z0-9._-]{1,128}$/) })) })
const input = computed(() => inputSchema.safeParse({ source: { url: draft.url, token: draft.token }, sourceBackupReference: draft.sourceBackupReference, monitoring: monitoring.value }))
const names = computed(() => new Map(props.nodes.map(node => [node.id, node.name])))
const canPreview = computed(() => ['none', 'preview'].includes(migration.state.value?.phase ?? 'none'))
const digest = computed(() => migration.preview.value?.digest ?? migration.state.value?.snapshotDigest ?? '')
async function capture(): Promise<void> {
  if (jsonValid.value && input.value.success && await migration.capture(input.value.data)) draft.token = ''
}
type Checkpoint = XNodeMigration['nodes'][number]
const columns = computed<TableColumn<Checkpoint>[]>(() => [
 { accessorKey: 'nodeId', header: t('xnode.node') },
 { accessorKey: 'phase', header: t('xnodeMigration.ownership') },
 { id: 'actions', header: t('xnodeMigration.actions') },
])
function importSnapshot(): void { void migration.run(() => xnodeApi.importMigration(digest.value)) }
function moveNode(id: string, action: 'cutover' | 'rollback'): void { void migration.run(() => xnodeApi.migrateNode(id, action)) }
</script>

<template>
  <div class="migration-workspace">
    <header class="flex items-center justify-between gap-3"><h3>{{ t('xnodeMigration.title') }}</h3><UButton color="neutral" variant="ghost" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="migration.loading.value" @click="migration.load" /></header>
    <OperationStatusNotice :receipt="migration.operation.receipt.value" :error="migration.error.value ?? migration.operation.error.value" :checking="migration.operation.checking.value" @refresh="migration.operation.refresh" />
    <AdminSectionState :loading="migration.loading.value" :error="migration.state.value ? null : migration.error.value" @retry="migration.load">
      <p class="text-sm text-muted">{{ t('xnodeMigration.continuity') }}</p>
      <div v-if="canPreview" class="grid gap-4">
        <div class="migration-form">
          <UFormField :label="t('xnodeMigration.sourceUrl')"><UInput v-model="draft.url" type="url" autocomplete="off" class="w-full" :disabled="migration.busy.value" /></UFormField>
          <UFormField :label="t('xnodeMigration.sourceToken')"><UInput v-model="draft.token" type="password" autocomplete="new-password" class="w-full" :disabled="migration.busy.value" /></UFormField>
          <UFormField :label="t('xnodeMigration.sourceBackup')" class="md:col-span-2"><UInput v-model="draft.sourceBackupReference" class="w-full" :disabled="migration.busy.value" /></UFormField>
        </div>
        <XNodeJsonEditor v-model="monitoring" :label="t('xnodeMigration.monitoring')" :rows="4" :disabled="migration.busy.value" @valid="jsonValid = $event" />
        <div><UButton :label="t('xnodeMigration.preview')" :loading="migration.saving.value" :disabled="!input.success || !jsonValid || migration.busy.value" @click="capture" /></div>
      </div>
      <div v-if="digest" class="grid gap-3">
        <p class="text-xs break-all text-muted">{{ t('xnodeMigration.snapshot', { digest }) }}</p>
        <p v-if="migration.preview.value" class="text-sm">{{ t('xnodeMigration.counts', { nodes: migration.preview.value.nodes, groups: migration.preview.value.groups, accounts: migration.preview.value.accounts, endpoints: migration.preview.value.endpoints }) }}</p>
        <ul v-if="migration.preview.value?.issues.length" class="grid gap-2">
          <li v-for="(issue, index) in migration.preview.value.issues" :key="index" class="flex flex-wrap gap-2 text-sm"><UBadge :color="issue.blocking ? 'warning' : 'neutral'" variant="subtle">{{ t(issue.blocking ? 'xnodeMigration.requiresCorrection' : 'xnodeMigration.review') }}</UBadge><span>{{ t(`xnodeMigration.issues.${issue.code}`) }}</span><code>{{ issue.resourceId }}</code></li>
        </ul>
        <div v-if="['preview', 'importing'].includes(migration.state.value?.phase ?? '')"><UButton :label="t('xnodeMigration.import')" :disabled="migration.busy.value" :loading="migration.saving.value" @click="importSnapshot" /></div>
      </div>
      <p v-if="migration.state.value?.backupName" class="text-xs text-muted">{{ t('xnodeMigration.panelBackup', { name: migration.state.value.backupName }) }}</p>
      <p v-if="!canPreview && !migration.state.value?.nodes.length" class="text-muted">{{ t('xnodeMigration.empty') }}</p>
      <div v-if="migration.state.value?.nodes.length" class="overflow-x-auto">
        <UTable :data="migration.state.value.nodes" :columns="columns" class="w-full">
          <template #nodeId-cell="{ row }"><span>{{ names.get(row.original.nodeId) ?? row.original.nodeId }}</span><p v-if="row.original.errorCode" class="text-xs text-muted">{{ t('xnodeMigration.recoveryPending') }}</p></template>
          <template #phase-cell="{ row }"><UBadge color="neutral" variant="subtle">{{ t(`xnodeMigration.phases.${row.original.phase}`) }}</UBadge></template>
          <template #actions-cell="{ row }"><div class="flex gap-2"><UButton v-if="['legacy', 'preparing'].includes(row.original.phase)" size="sm" :label="t('xnodeMigration.cutover')" :disabled="migration.busy.value" @click="moveNode(row.original.nodeId, 'cutover')" /><UButton v-if="row.original.phase !== 'legacy'" size="sm" color="neutral" variant="outline" :label="t('xnodeMigration.rollback')" :disabled="migration.busy.value" @click="moveNode(row.original.nodeId, 'rollback')" /></div></template>
        </UTable>
      </div>
      <p v-if="migration.state.value?.nodes.length" class="text-xs text-muted">{{ t('xnodeMigration.signedAgent') }}</p>
    </AdminSectionState>
  </div>
</template>

<style scoped>
.migration-workspace { display: grid; grid-template-columns: minmax(0,1fr); gap: 20px; min-width: 0; }
.migration-workspace > * { min-width: 0; }
.migration-form { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 16px; }
@media(max-width:640px) { .migration-form { grid-template-columns: minmax(0,1fr); } }
</style>
