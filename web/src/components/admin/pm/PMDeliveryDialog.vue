<script setup lang="ts">
import './modal.css'
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import type { PMConversation, PMDelivery } from '@/api/types'
import { pmApi } from '@/api/pm'
import InlineNotice from '@/components/common/InlineNotice.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { localizedError, useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import { createLatestRequest } from '@/utils/latestRequest'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { pmTopicLink } from './links'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ conversation: PMConversation | null }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const items = shallowRef<PMDelivery[]>([])
const loading = shallowRef(false)
const error = shallowRef<string | null>(null)
const latest = createLatestRequest()
const columns = computed(() => [{ id: 'direction', header: t('adminWorkspace.type') }, { id: 'created', header: t('adminWorkspace.created') }, { accessorKey: 'sourceMessageId', header: t('adminWorkspace.identity') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'read', header: t('adminWorkspace.readStatus') }, { id: 'actions', header: t('adminWorkspace.actions') }])
function deliveryError(item: PMDelivery): string {
  const key = `pm.errors.${item.errorCode}`
  const value = t(key)
  return value === key ? t('pm.deliveryFailed') : value
}
async function load(): Promise<void> {
  if (!props.conversation) return
  const token = latest.begin()
  loading.value = true; error.value = null
  try { const result = await pmApi.deliveries(props.conversation.id); if (latest.isCurrent(token)) items.value = result.items }
  catch (caught) { if (latest.isCurrent(token)) error.value = localizedError(caught, 'pm.loadFailed') }
  finally { if (latest.isCurrent(token)) loading.value = false }
}
watch(open, next => { if (next) { items.value = []; void load() } else latest.invalidate() })
onScopeDispose(latest.dispose)
function view(item: PMDelivery): void {
  if (!props.conversation) return
  const link = pmTopicLink(props.conversation.chatId, item.topicId, item.direction === 'outbound' ? item.sourceMessageId : item.resultMessageId)
  if (link) globalThis.open(link, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <UModal v-model:open="open" :title="t('pm.deliveries')" :description="t('pm.reviewHint')" :ui="{ content: reducedMotion ? 'max-w-4xl pm-modal--reduced' : 'max-w-4xl', overlay: reducedMotion ? 'pm-modal--reduced' : '' }">
    <template #body>
      <div class="pm-deliveries__toolbar"><UButton color="neutral" variant="ghost" icon="i-ph-arrow-clockwise" :label="t('pm.refresh')" :loading="loading" :disabled="loading" @click="load" /></div>
      <USkeleton v-if="loading" class="h-24" />
      <div v-else-if="error"><InlineNotice tone="warning">{{ error }}</InlineNotice><UButton color="neutral" variant="ghost" :label="t('pm.retry')" @click="load" /></div>
      <UTable v-else class="admin-table" :data="items" :columns="columns">
        <template #direction-cell="{ row }">{{ t(`pm.direction.${row.original.direction}`) }}</template>
        <template #created-cell="{ row }">{{ formatDateTime(row.original.createdAt) }}</template>
        <template #status-cell="{ row }"><div class="admin-cell"><StatusBadge :tone="row.original.status === 'succeeded' ? 'success' : row.original.status === 'failed' ? 'danger' : 'warning'" :label="t(`operations.status.${row.original.status}`)" /><small v-if="row.original.errorCode">{{ deliveryError(row.original) }}</small></div></template>
        <template #read-cell="{ row }"><div v-if="row.original.direction === 'outbound' && row.original.status === 'succeeded'" class="admin-cell"><StatusBadge :tone="row.original.readAt ? 'success' : 'neutral'" :label="row.original.readAt ? t(`pm.readState.${row.original.readSource}`) : t('pm.unread')" /><small v-if="row.original.readAt">{{ formatDateTime(row.original.readAt) }}</small></div></template>
        <template #actions-cell="{ row }"><UButton v-if="row.original.topicId && (row.original.direction === 'outbound' || row.original.resultMessageId)" color="neutral" variant="ghost" icon="i-ph-arrow-square-out" :label="t('pm.viewInTelegram')" @click="view(row.original)" /></template>
        <template #empty>{{ t('pm.noDeliveries') }}</template>
      </UTable>
    </template>
  </UModal>
</template>

<style scoped>.pm-deliveries__toolbar { display: flex; justify-content: end; margin-bottom: 1rem; }</style>
