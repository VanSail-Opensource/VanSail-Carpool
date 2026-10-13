<script setup lang="ts">
import { computed } from 'vue'
import type { TableColumn } from '@nuxt/ui'
import type { JobRecord } from '@/api/types'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import StatusBadge from '@/components/common/StatusBadge.vue'

defineProps<{ items: readonly JobRecord[]; busy: boolean; activeId: string | null }>()
defineEmits<{ retry: [id: string]; delete: [job: JobRecord] }>()
const { t } = useI18n()
const columns = computed<TableColumn<JobRecord>[]>(() => [
  { accessorKey: 'kind', header: t('browser.table.kind') },
  { accessorKey: 'status', header: t('browser.table.status') },
  { accessorKey: 'attempts', header: t('browser.table.attempts') },
  { accessorKey: 'availableAt', header: t('browser.table.date') },
  { id: 'actions', header: t('browser.table.actions') },
])
function scheduled(job: JobRecord): boolean { return job.status === 'pending' && Date.parse(job.availableAt) > Date.now() }
</script>

<template>
  <UTable :data="items" :columns="columns" class="admin-table">
    <template #kind-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.kind }}</strong><small v-if="row.original.lastError">{{ row.original.lastError }}</small></div></template>
    <template #status-cell="{ row }"><StatusBadge :tone="row.original.status === 'done' ? 'success' : row.original.status === 'failed' ? 'danger' : 'warning'" :label="t(scheduled(row.original) ? 'adminBackups.status.scheduled' : `adminBackups.status.${row.original.status}`)" /></template>
    <template #availableAt-cell="{ row }">{{ formatDateTime(row.original.status === 'pending' ? row.original.availableAt : row.original.updatedAt) }}</template>
    <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton v-if="row.original.status === 'failed'" color="neutral" variant="ghost" icon="i-ph-arrow-clockwise" :disabled="busy" :loading="activeId === row.original.id" :label="t('adminBackups.retry')" @click="$emit('retry', row.original.id)" /><UButton color="error" variant="ghost" icon="i-ph-trash" :disabled="busy || row.original.status === 'processing'" :aria-label="t('adminBackups.deleteJobLabel', { kind: row.original.kind })" @click="$emit('delete', row.original)" /></div></template>
    <template #empty><div class="empty-inline"><h3>{{ t('adminBackups.noJobs') }}</h3><p>{{ t('adminBackups.noJobsHint') }}</p></div></template>
  </UTable>
</template>
