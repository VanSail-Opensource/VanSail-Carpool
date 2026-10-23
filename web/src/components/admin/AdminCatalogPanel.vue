<script setup lang="ts">
import { computed, shallowRef } from 'vue'

import { featuresApi, type AdminStatistics, type StatisticsQuery } from '@/api/features'
import type { Combo, SquadProduct, SquadProductWrite } from '@/api/types'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import SquadProfileSummary from '@/components/squad-profile/SquadProfileSummary.vue'
import { useAdminSection } from '@/composables/useAdminSection'
import { useI18n } from '@/i18n'
import { formatBytes, formatMoney } from '@/utils/format'
import AdminCatalogEditor from './AdminCatalogEditor.vue'
import AdminSectionState from './AdminSectionState.vue'
import AdminSquadEditor from './AdminSquadEditor.vue'
import AdminStatisticsPanel from './AdminStatisticsPanel.vue'

const combos = useAdminSection<Combo>('combos')
const squads = useAdminSection<SquadProduct>('squad-products')
const editing = shallowRef<Combo | null | undefined>(undefined)
const deleting = shallowRef<Combo | null>(null)
const editingSquad = shallowRef<SquadProduct | null>(null)
const statisticsTarget = shallowRef<{ kind: 'combo' | 'squad'; id: string; title: string } | null>(null)
const { t } = useI18n()
const activeTab = shallowRef('combos')
const tabs = computed(() => [{ value: 'combos', label: t('adminNav.catalog') }, { value: 'squads', label: t('adminCatalog.squads') }])
const columns = computed(() => [{ id: 'name', header: t('adminWorkspace.name') }, { id: 'terms', header: t('adminWorkspace.details') }, { id: 'price', header: t('adminWorkspace.price') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }])

function loadStatistics(query: StatisticsQuery): Promise<AdminStatistics> {
  const target = statisticsTarget.value
  if (!target) return Promise.reject(new Error(t('adminCatalog.chooseRecord')))
  return target.kind === 'combo'
    ? featuresApi.getAdminComboStatistics(target.id, query)
    : featuresApi.getAdminSquadStatistics(target.id, query)
}

async function save(payload: Record<string, unknown>): Promise<void> {
  const success = editing.value?.id
    ? await combos.update(editing.value.id, payload)
    : await combos.create(payload)
  if (success) editing.value = undefined
}

async function remove(): Promise<void> {
  if (!deleting.value) return
  const success = await combos.remove(deleting.value.id)
  if (success) deleting.value = null
}

function importSquads(): void {
  void squads.perform(() => import('@/api/client').then(({ api }) => api.importAdminSquadProducts()))
}

async function saveSquad(payload: SquadProductWrite): Promise<void> {
  if (!editingSquad.value) return
  const id = editingSquad.value.id
  const success = await squads.perform(() => import('@/api/client').then(({ api }) => api.updateAdminSquadProduct(id, payload)))
  if (success) editingSquad.value = null
}
</script>

<template>
  <section class="admin-panel">
    <div class="admin-panel__heading"><div><h2>{{ t('adminCatalog.title') }}</h2><p>{{ t('adminCatalog.copy') }}</p></div><div class="row-actions"><UButton v-if="activeTab === 'combos'" icon="i-ph-plus" :label="t('adminCatalog.newCombo')" @click="editing = null" /><UButton v-else color="neutral" variant="outline" icon="i-ph-arrow-clockwise" :loading="squads.busy.value" :disabled="squads.busy.value" :label="t('adminCatalog.refreshSquads')" @click="importSquads" /></div></div>
    <UTabs v-model="activeTab" :items="tabs" variant="link" :content="false" class="admin-workspace-tabs" />
    <div v-show="activeTab === 'combos'">
      <AdminSectionState :loading="combos.loading.value" :error="combos.error.value" @retry="combos.load()">
        <UTable class="admin-table" :data="combos.items.value" :columns="columns">
          <template #name-cell="{ row }"><strong>{{ row.original.name }}</strong></template>
          <template #terms-cell="{ row }"><div class="admin-cell"><span>{{ formatBytes(row.original.trafficLimitBytes) }}</span><small>{{ t('adminCatalog.comboSummary', { traffic: formatBytes(row.original.trafficLimitBytes), days: row.original.validityDays }) }}</small></div></template>
          <template #price-cell="{ row }">{{ formatMoney(row.original.price) }}</template>
          <template #status-cell="{ row }"><StatusBadge :tone="row.original.active ? 'success' : 'neutral'" :label="t(row.original.active ? 'adminCatalog.available' : 'adminCatalog.paused')" /></template>
          <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-chart-bar" :aria-label="t('adminCatalog.statisticsFor', { name: row.original.name })" @click="statisticsTarget = { kind: 'combo', id: row.original.id, title: t('adminCatalog.statisticsTitle', { name: row.original.name }) }" /><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminCatalog.editNamed', { name: row.original.name })" @click="editing = row.original" /><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminCatalog.hideNamed', { name: row.original.name })" @click="deleting = row.original" /></div></template>
          <template #empty>{{ t('adminCatalog.noCombos') }}</template>
        </UTable>
      </AdminSectionState>
    </div>
    <div v-show="activeTab === 'squads'">
      <AdminSectionState :loading="squads.loading.value" :error="squads.error.value" @retry="squads.load()">
        <UTable class="admin-table" :data="squads.items.value" :columns="columns">
          <template #name-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.name }}</strong><code>{{ row.original.groupId }}</code></div></template>
          <template #terms-cell="{ row }"><SquadProfileSummary :profile="row.original.profile" :description="row.original.description" compact /></template>
          <template #price-cell="{ row }">{{ formatMoney(row.original.price) }}</template>
          <template #status-cell="{ row }"><StatusBadge :tone="row.original.upstreamPresent && row.original.visible ? 'success' : 'neutral'" :label="t(row.original.upstreamPresent && row.original.visible ? 'adminCatalog.available' : 'adminCatalog.hidden')" /></template>
          <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-chart-bar" :aria-label="t('adminCatalog.statisticsFor', { name: row.original.name })" @click="statisticsTarget = { kind: 'squad', id: row.original.id, title: t('adminCatalog.statisticsTitle', { name: row.original.name }) }" /><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminCatalog.editNamed', { name: row.original.name })" @click="editingSquad = row.original" /></div></template>
          <template #empty>{{ t('adminCatalog.noSquads') }}</template>
        </UTable>
      </AdminSectionState>
    </div>
    <AdminCatalogEditor v-if="editing !== undefined" :combo="editing ?? undefined" :squads="squads.items.value" :busy="combos.busy.value" :error="combos.error.value" @save="save" @cancel="editing = undefined" />
    <AdminSquadEditor v-if="editingSquad" :squad="editingSquad" :busy="squads.busy.value" :error="squads.error.value" @save="saveSquad" @cancel="editingSquad = null" />
    <USlideover :open="statisticsTarget !== null" :title="statisticsTarget?.title" @update:open="!$event && (statisticsTarget = null)"><template #body><AdminStatisticsPanel v-if="statisticsTarget" :title="statisticsTarget.title" :load="loadStatistics" @close="statisticsTarget = null" /></template></USlideover>
    <ConfirmDialog :open="deleting !== null" :title="t('adminCatalog.hideTitle')" :description="t('adminCatalog.hideDescription')" :confirm-label="t('adminCatalog.hideCombo')" :busy="combos.busy.value" danger @update:open="!$event && (deleting = null)" @confirm="remove" />
  </section>
</template>
