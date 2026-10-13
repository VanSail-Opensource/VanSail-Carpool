<script setup lang="ts">
import { computed } from 'vue'
import type { TableColumn } from '@nuxt/ui'
import type { BackupRecord } from '@/api/types'
import { useI18n } from '@/i18n'
import { formatBytes, formatDateTime } from '@/utils/format'
import StatusBadge from '@/components/common/StatusBadge.vue'

defineProps<{ items: readonly BackupRecord[]; busy: boolean }>()
defineEmits<{ download: [backup: BackupRecord]; restore: [backup: BackupRecord]; delete: [backup: BackupRecord] }>()
const { t } = useI18n()
const columns = computed<TableColumn<BackupRecord>[]>(() => [
  { accessorKey: 'path', header: t('browser.table.name') },
  { accessorKey: 'sizeBytes', header: t('browser.table.size') },
  { accessorKey: 'createdAt', header: t('browser.table.date') },
  { accessorKey: 'status', header: t('browser.table.status') },
  { id: 'actions', header: t('browser.table.actions') },
])
function name(backup: BackupRecord): string { return backup.path.split(/[\\/]/).pop() || t('adminBackups.backupName', { id: backup.id }) }
</script>

<template>
  <UTable :data="items" :columns="columns" class="admin-table">
    <template #path-cell="{ row }"><div class="admin-cell"><strong>{{ name(row.original) }}</strong><code>{{ row.original.id }}</code></div></template>
    <template #sizeBytes-cell="{ row }">{{ formatBytes(row.original.sizeBytes) }}</template>
    <template #createdAt-cell="{ row }">{{ formatDateTime(row.original.createdAt) }}</template>
    <template #status-cell="{ row }"><StatusBadge :tone="row.original.status === 'complete' ? 'success' : 'warning'" :label="t(row.original.status === 'complete' ? 'adminBackups.verified' : `adminBackups.status.${row.original.status}`)" /></template>
    <template #actions-cell="{ row }"><div v-if="row.original.status === 'complete'" class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-download-simple" :disabled="busy" :aria-label="t('adminBackups.download')" @click="$emit('download', row.original)" /><UButton color="neutral" variant="ghost" icon="i-ph-arrow-counter-clockwise" :disabled="busy" :aria-label="t('adminBackups.restore')" @click="$emit('restore', row.original)" /><UButton color="error" variant="ghost" icon="i-ph-trash" :disabled="busy" :aria-label="t('adminBackups.delete')" @click="$emit('delete', row.original)" /></div></template>
    <template #empty><div class="empty-inline"><h3>{{ t('adminBackups.none') }}</h3><p>{{ t('adminBackups.noneHint') }}</p></div></template>
  </UTable>
</template>
