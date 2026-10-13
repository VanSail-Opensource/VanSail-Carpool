<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { TableColumn } from '@nuxt/ui'
import type { USDTReviewPage, USDTTransfer } from '@/api/usdt'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import { paymentChannelLabel } from '@/components/billing/paymentOptions'
import { networkRails } from './configForm'
import InlineNotice from '@/components/common/InlineNotice.vue'
import USDTReviewItem from './USDTReviewItem.vue'
defineProps<{ page: USDTReviewPage; resolving: string | null; error?: string | null }>()
const emit = defineEmits<{ refresh: []; more: []; resolve: [id: string, orderId: string, reason: string] }>()
const { t } = useI18n()
const selected = shallowRef<USDTTransfer | null>(null)
function networkLabel(transfer: USDTTransfer): string { return paymentChannelLabel(networkRails[transfer.network], transfer.network) }
const columns = computed<TableColumn<USDTTransfer>[]>(() => [{ accessorKey: 'amount', header: t('adminWorkspace.amount') }, { id: 'network', header: t('adminUsdt.network') }, { accessorKey: 'recipient', header: t('adminUsdt.recipient') }, { id: 'date', header: t('adminWorkspace.date') }, { id: 'reason', header: t('adminUsdt.reason') }, { id: 'actions', header: t('adminWorkspace.actions') }])
</script>
<template>
  <section class="usdt-review">
    <div class="admin-subsection-heading"><div><h3>{{ t('adminUsdt.review') }}</h3><p>{{ t('adminUsdt.reviewHint') }}</p></div><UButton color="neutral" variant="ghost" icon="i-ph-arrows-clockwise" :aria-label="t('common.refresh')" @click="emit('refresh')" /></div>
    <UTable class="admin-table" :data="page.items" :columns="columns">
      <template #amount-cell="{ row }">{{ row.original.amount }} {{ t('adminUsdt.currency') }}</template>
      <template #network-cell="{ row }">{{ networkLabel(row.original) }}</template>
      <template #date-cell="{ row }">{{ formatDateTime(row.original.occurredAt) }}</template>
      <template #reason-cell="{ row }">{{ t(`adminUsdt.reviewReasons.${row.original.reason}`) }}</template>
      <template #actions-cell="{ row }"><UButton color="neutral" variant="ghost" icon="i-ph-magnifying-glass" :label="t('adminWorkspace.details')" @click="selected = row.original" /></template>
      <template #empty>{{ t('adminUsdt.noTransfers') }}</template>
    </UTable>
    <div v-if="page.nextCursor" class="admin-pagination"><UButton color="neutral" variant="outline" :label="t('adminUsdt.more')" @click="emit('more')" /></div>
    <USlideover :open="selected !== null" :title="t('adminUsdt.review')" :description="t('adminUsdt.reviewHint')" :dismissible="!resolving" @update:open="!$event && (selected = null)"><template #body><InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice><USDTReviewItem v-if="selected" :key="selected.id" :transfer="selected" :busy="resolving === selected.id" @resolve="(id, order, reason) => emit('resolve', id, order, reason)" /></template></USlideover>
  </section>
</template>
