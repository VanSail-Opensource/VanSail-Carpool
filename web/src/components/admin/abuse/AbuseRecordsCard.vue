<script setup lang="ts">
import { computed } from 'vue'
import type { AbuseRecord } from '@/api/abuse'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ records: AbuseRecord[]; hasMore: boolean; loadingMore: boolean }>()
const emit = defineEmits<{ loadMore: [] }>()
const { t } = useI18n()
const columns = computed(() => [
  { accessorKey: 'occurredAt', header: t('adminAbuse.occurredAt') },
  { accessorKey: 'username', header: t('adminAbuse.username') },
  { accessorKey: 'reason', header: t('adminAbuse.reason') },
  { accessorKey: 'qps', header: t('adminAbuse.qps') },
  { accessorKey: 'action', header: t('adminAbuse.action') },
])
const tableRows = computed(() => props.records.map(record => ({ ...record, username: record.username || t('adminAbuse.unknownUsername'), occurredAt: formatDateTime(record.occurredAt), qps: `${record.measuredQPS} / ${record.qpsLimit}`, action: t(`abuse.action.${record.action}`) })))
</script>

<template>
  <section class="card records-card">
    <div>
      <p class="eyebrow">{{ t('adminAbuse.recordsEyebrow') }}</p>
      <h3>{{ t('adminAbuse.recordsTitle') }}</h3>
    </div>
    <UTable class="admin-table" :data="tableRows" :columns="columns"><template #empty>{{ t('adminAbuse.emptyRecords') }}</template></UTable>
    <UButton v-if="hasMore" color="neutral" variant="outline" :loading="loadingMore" :label="t('adminAbuse.loadMoreRecords')" @click="emit('loadMore')" />
  </section>
</template>

<style scoped>.card { display: grid; gap: 1rem; padding: 0; }</style>
