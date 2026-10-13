<script setup lang="ts">
import { computed } from 'vue'
import type { AdminUserDetail } from '@/api/adminOperations'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
type IPBlock = AdminUserDetail['ipBlocks'][number]
defineProps<{ items: readonly IPBlock[]; busy: boolean }>()
const emit = defineEmits<{ unblock: [block: IPBlock] }>()
const { t } = useI18n()
const columns = computed(() => [{ accessorKey: 'ip', header: t('adminUserProfile.ipBlocks') }, { id: 'expiry', header: t('adminWorkspace.validity') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }])
</script>
<template>
  <section class="admin-section">
    <div class="admin-subsection-heading"><div><h3>{{ t('adminUserProfile.ipBlocks') }}</h3><p>{{ t('adminUserProfile.ipBlocksHint') }}</p></div></div>
    <UTable class="admin-table" :data="items" :columns="columns">
      <template #expiry-cell="{ row }">{{ formatDateTime(row.original.expiresAt) }}</template>
      <template #status-cell="{ row }"><StatusBadge :tone="row.original.status === 'active' ? 'success' : 'warning'" :label="t(`adminUserProfile.ipBlockStatus.${row.original.status}`)" /></template>
      <template #actions-cell="{ row }"><UButton color="neutral" variant="ghost" icon="i-ph-shield-check" :label="t('adminUserProfile.unblockIP')" :disabled="busy || row.original.status === 'unblocking'" @click="emit('unblock', row.original)" /></template>
      <template #empty>{{ t('adminUserProfile.noIPBlocks') }}</template>
    </UTable>
  </section>
</template>
