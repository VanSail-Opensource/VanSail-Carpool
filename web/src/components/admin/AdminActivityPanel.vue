<script setup lang="ts">
import { useActivityWorkspace } from './activity/useActivityWorkspace'
import AdminLuckyDrawEditor from './activity/AdminLuckyDrawEditor.vue'
import AdminStatisticsPanel from './AdminStatisticsPanel.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { txbInputFromMinor } from '@/utils/format'
const { games, draws, editingGameId, editingDraw, loading, busy, error, gameDraft, statisticsTarget, deleting, t, activeTab, tabs, gameColumns, drawColumns, iconItems, comboItems, squadItems, loadStatistics, removeActivity, editGame, saveGame, saveDraw, publishDraw } = useActivityWorkspace()
</script>

<template>
  <section class="admin-panel">
    <div class="admin-panel__heading">
      <div><h2>{{ t('adminActivityManagement.title') }}</h2><p>{{ t('adminActivityManagement.copy') }}</p></div>
    </div>

    <UTabs v-model="activeTab" :items="tabs" :content="false" variant="link" class="admin-workspace-tabs" />
    <section v-show="activeTab === 'games'" class="activity-admin-section">
      <div class="activity-admin-section__heading"><div><h3>{{ t('adminActivityManagement.games') }}</h3><p>{{ t('adminActivityManagement.gamesHint') }}</p></div><UButton icon="i-ph-plus" :label="t('adminActivityManagement.newGame')" @click="editGame()" /></div>
      <USlideover :open="editingGameId !== undefined" :title="editingGameId ? t('adminActivityManagement.editGame') : t('adminActivityManagement.newGame')" :description="t('adminActivityManagement.gamesHint')" :dismissible="!busy" @update:open="!$event && (editingGameId = undefined)">
        <template #body>
          <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
          <form class="admin-editor-form" @submit.prevent="saveGame">
            <div class="catalog-editor__heading"><h3>{{ editingGameId ? t('adminActivityManagement.editGame') : t('adminActivityManagement.newGame') }}</h3></div>
            <UFormField name="game-name" :label="t('adminActivityManagement.name')" required><UInput v-model.trim="gameDraft.name" class="w-full" :maxlength="80" /></UFormField>
            <UFormField name="game-icon" :label="t('adminActivityManagement.icon')"><USelect v-model="gameDraft.icon" class="w-full" :items="iconItems" /></UFormField>
            <UFormField class="catalog-editor__wide" name="game-description" :label="t('adminActivityManagement.description')"><UTextarea v-model.trim="gameDraft.description" class="w-full" :rows="2" :maxlength="300" /></UFormField>
            <UFormField name="win-chance" :label="t('adminActivityManagement.winChance')" required><UInput v-model.number="gameDraft.winChancePercent" class="w-full" type="number" :min="0.01" :max="99.99" :step="0.01" /></UFormField>
            <UFormField name="return-multiplier" :label="t('adminActivityManagement.returnMultiplier')" required><UInput v-model.number="gameDraft.returnMultiplier" class="w-full" type="number" :min="1.01" :step="0.01" /></UFormField>
            <TxbAmountField id="game-min-stake" v-model="gameDraft.minStake" :label="t('adminActivityManagement.minimumStake')" min-minor="1" required />
            <TxbAmountField id="game-max-stake" v-model="gameDraft.maxStake" :label="t('adminActivityManagement.maximumStake')" min-minor="1" required />
            <SwitchField id="game-enabled" v-model="gameDraft.enabled" :label="t('adminActivityManagement.available')" :help="t('adminActivityManagement.availableHint')" />
            <div class="button-row"><UButton color="neutral" variant="outline" :label="t('common.cancel')" @click="editingGameId = undefined" /><UButton type="submit" :loading="busy" :disabled="busy" :label="busy ? t('common.saving') : t('adminActivityManagement.saveGame')" /></div>
          </form>
        </template>
      </USlideover>
      <UTable class="admin-table" :data="games" :columns="gameColumns" :loading="loading">
        <template #loading><div class="admin-loading" role="status"><USkeleton class="h-10 w-full" /><span>{{ t('common.loading') }}</span></div></template>
        <template #terms-cell="{ row }">{{ t('adminActivityManagement.gameSummary', { chance: (row.original.winChanceBps / 100).toFixed(2), multiplier: (row.original.returnMultiplierBps / 10000).toFixed(2), minimum: txbInputFromMinor(row.original.minimumStakeMinor), maximum: txbInputFromMinor(row.original.maximumStakeMinor) }) }}</template>
        <template #status-cell="{ row }"><UBadge :color="row.original.enabled ? 'success' : 'neutral'" :label="t(row.original.enabled ? 'common.active' : 'adminCatalog.paused')" variant="subtle" /></template>
        <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-chart-bar" :aria-label="t('adminActivityManagement.statisticsFor', { name: row.original.name })" @click="statisticsTarget = { kind: 'game', id: row.original.id, title: t('adminActivityManagement.statisticsTitle', { name: row.original.name }) }" /><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminActivityManagement.editNamed', { name: row.original.name })" @click="editGame(row.original)" /><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminActivityManagement.deleteNamed', { name: row.original.name })" @click="deleting = { kind: 'game', id: row.original.id, name: row.original.name }" /></div></template>
        <template #empty>{{ t('adminActivityManagement.noGames') }}</template>
      </UTable>
      <AdminStatisticsPanel v-if="statisticsTarget?.kind === 'game'" :title="statisticsTarget.title" :load="loadStatistics" @close="statisticsTarget = null" />
    </section>

    <section v-show="activeTab === 'draws'" class="activity-admin-section">
      <div class="activity-admin-section__heading"><div><h3>{{ t('adminActivityManagement.draws') }}</h3><p>{{ t('adminDrawRedesign.drawsHint') }}</p></div><UButton icon="i-ph-plus" :label="t('adminActivityManagement.newDraw')" @click="editingDraw = null" /></div>
      <AdminLuckyDrawEditor v-if="editingDraw !== undefined" :draw="editingDraw" :combo-items="comboItems" :squad-items="squadItems" :busy="busy" :error="error" @save="saveDraw" @cancel="editingDraw = undefined" />
      <UTable class="admin-table" :data="draws" :columns="drawColumns" :loading="loading">
        <template #loading><div class="admin-loading" role="status"><USkeleton class="h-10 w-full" /><span>{{ t('common.loading') }}</span></div></template>
        <template #type-cell="{ row }"><div class="admin-cell"><span>{{ t('adminDrawRedesign.' + row.original.kind) }}</span><small v-if="row.original.kind === 'raffle'">{{ t('adminDrawRedesign.currentSeats', { seats: row.original.seats, threshold: row.original.threshold ?? 0 }) }}</small></div></template>
        <template #price-cell="{ row }">{{ txbInputFromMinor(row.original.feeTxbMinor) }} TXB</template>
        <template #status-cell="{ row }"><UBadge color="neutral" variant="subtle" :label="t('adminDrawRedesign.' + row.original.status)" /></template>
        <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton v-if="row.original.kind === 'raffle' && row.original.status === 'draft'" variant="ghost" icon="i-ph-paper-plane-tilt" :aria-label="t('adminDrawRedesign.publish')" :disabled="busy" @click="publishDraw(row.original.id)" /><UButton color="neutral" variant="ghost" icon="i-ph-chart-bar" :aria-label="t('adminActivityManagement.statisticsFor', { name: row.original.name })" @click="statisticsTarget = { kind: 'draw', id: row.original.id, title: t('adminActivityManagement.statisticsTitle', { name: row.original.name }) }" /><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminActivityManagement.editNamed', { name: row.original.name })" @click="editingDraw = row.original" /><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminActivityManagement.deleteNamed', { name: row.original.name })" @click="deleting = { kind: 'draw', id: row.original.id, name: row.original.name }" /></div></template>
        <template #empty>{{ t('adminActivityManagement.noDraws') }}</template>
      </UTable>
      <AdminStatisticsPanel v-if="statisticsTarget?.kind === 'draw'" :title="statisticsTarget.title" :load="loadStatistics" @close="statisticsTarget = null" />
    </section>

    <UAlert v-if="error" class="admin-error" color="warning" variant="soft" icon="i-ph-warning" :description="error" />
    <ConfirmDialog :open="Boolean(deleting)" :title="t('adminActivityManagement.deleteTitle', { name: deleting?.name ?? t('adminActivityManagement.activity') })" :description="deleting?.kind === 'draw' ? t('adminDrawRedesign.deleteHint') : t('adminActivityManagement.deleteDescription')" :confirm-label="t('adminActivityManagement.deletePermanently')" :busy="busy" danger @update:open="!$event && (deleting = null)" @confirm="removeActivity" />
  </section>
</template>

<style scoped>
.activity-admin-section { display: grid; gap: 0.8rem; padding: 1rem; border-top: 1px solid var(--line); }
.activity-admin-section__heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 0.8rem; }
.activity-admin-section__heading h3, .activity-admin-section__heading p { margin: 0; }
.activity-admin-section__heading p { margin-top: 0.25rem; color: var(--text-muted); font-size: 0.78rem; }
.admin-list-row__icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 10px; color: var(--accent); background: var(--accent-soft); }
.admin-error { margin: 1rem; }
@media (max-width: 719px) {
  .activity-admin-section__heading { display: grid; gap: 0.65rem; }
  .activity-admin-section__heading > .button { width: 100%; }
}
</style>
