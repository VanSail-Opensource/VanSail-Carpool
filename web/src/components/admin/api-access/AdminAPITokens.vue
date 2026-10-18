<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { TableColumn } from '@nuxt/ui'
import type { PanelAPIToken } from '@/api/adminAPITokens'
import type { PanelAPITokenWrite } from '@/api/adminAPITokens'
import { getLocale, useI18n } from '@/i18n'
import AdminSectionState from '../AdminSectionState.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import APITokenForm from './APITokenForm.vue'
import APITokenSecret from './APITokenSecret.vue'
import { useAdminAPITokens } from './useAdminAPITokens'

const { t } = useI18n()
const state = useAdminAPITokens()
const creating = shallowRef(false)
async function create(value: PanelAPITokenWrite): Promise<void> {
  await state.create(value)
  if (state.issued.value) creating.value = false
}
const dates = computed(() => new Intl.DateTimeFormat(getLocale(), { dateStyle: 'medium', timeStyle: 'short' }))
const columns = computed<TableColumn<PanelAPIToken>[]>(() => [
  { accessorKey: 'name', header: t('apiTokens.name') }, { id: 'scopes', header: t('apiTokens.permissions') },
  { id: 'status', header: t('apiTokens.status') }, { id: 'activity', header: t('apiTokens.lastUsed') }, { id: 'actions', header: t('apiTokens.actions') },
])
function status(item: PanelAPIToken): string { return item.revokedAt ? 'revoked' : item.expiresAt && Date.parse(item.expiresAt) <= Date.now() ? 'expired' : 'active' }
const example = 'curl -H "Authorization: Bearer $API_TOKEN" "$PANEL_URL/api/v1/admin/xnode/nodes"'
</script>

<template>
  <section class="api-token-workspace">
    <header class="admin-section-header flex items-start justify-between gap-3"><div><h2>{{ t('apiTokens.title') }}</h2><p class="text-muted">{{ t('apiTokens.description') }}</p></div><UButton color="neutral" variant="ghost" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="state.loading.value" :disabled="state.busy.value" @click="state.load" /></header>
    <InlineNotice v-if="state.error.value || state.loadError.value && state.items.value.length" tone="warning">{{ state.error.value ?? state.loadError.value }}</InlineNotice>
    <APITokenSecret v-if="state.issued.value" :key="state.issued.value.item.id" :issued="state.issued.value" @dismiss="state.dismiss" />
    <div><UButton icon="i-ph-plus" :label="t('apiTokens.create')" :disabled="state.busy.value" @click="state.error.value = null; creating = true" /></div>
    <UModal v-model:open="creating" :title="t('apiTokens.create')" :description="t('apiTokens.permissionsHint')" :dismissible="!state.busy.value"><template #body><InlineNotice v-if="state.error.value" tone="warning">{{ state.error.value }}</InlineNotice><APITokenForm v-if="creating" :busy="state.busy.value" @create="create" /></template></UModal>
    <AdminSectionState :loading="state.loading.value" :error="state.items.value.length ? null : state.loadError.value" @retry="state.load">
      <div class="overflow-x-auto min-w-0">
        <UTable :data="state.items.value" :columns="columns">
          <template #name-cell="{ row }"><span>{{ row.original.name }}</span><p class="font-mono text-xs text-muted">{{ row.original.prefix }}…</p></template>
          <template #scopes-cell="{ row }"><div class="flex flex-wrap gap-1 max-w-64"><UBadge v-for="scope in row.original.scopes" :key="scope" variant="subtle" color="neutral">{{ t(`apiTokens.scopes.${scope}`) }}</UBadge></div></template>
          <template #status-cell="{ row }"><UBadge :color="status(row.original) === 'active' ? 'success' : 'neutral'" variant="subtle">{{ t(`apiTokens.states.${status(row.original)}`) }}</UBadge><p class="text-xs text-muted">{{ row.original.expiresAt ? dates.format(new Date(row.original.expiresAt)) : t('apiTokens.never') }}</p></template>
          <template #activity-cell="{ row }">{{ row.original.lastUsedAt ? dates.format(new Date(row.original.lastUsedAt)) : t('apiTokens.notUsed') }}</template>
          <template #actions-cell="{ row }"><UButton v-if="!row.original.revokedAt" color="error" variant="ghost" :label="t('apiTokens.revoke')" :disabled="state.busy.value" @click="state.revoke(row.original.id)" /></template>
          <template #empty>{{ t('apiTokens.empty') }}</template>
        </UTable>
      </div>
    </AdminSectionState>
    <section class="grid gap-2 min-w-0"><h3 class="font-medium">{{ t('apiTokens.usage') }}</h3><p class="text-sm text-muted">{{ t('apiTokens.usageHint') }}</p><code class="block whitespace-pre-wrap break-all text-xs">{{ example }}</code></section>
  </section>
</template>

<style scoped>
.api-token-workspace { display: grid; grid-template-columns: minmax(0, 1fr); gap: 24px; min-width: 0; }
.api-token-workspace > * { min-width: 0; }
</style>
