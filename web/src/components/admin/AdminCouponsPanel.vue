<script setup lang="ts">
import { computed } from 'vue'
import type { CouponDefinition } from '@/api/features'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useI18n } from '@/i18n'
import { useAdminCoupons } from './useAdminCoupons'

const { items, loading, refreshing, busy, error, editingId, deactivating, draft, comboItems, squadItems, optionsLoading, optionsError, load, loadOptions, edit, close, requestDeactivation, closeDeactivation, save, deactivate } = useAdminCoupons()
const { t } = useI18n()
const columns = computed(() => [{ accessorKey: 'code', header: t('adminCoupons.code') }, { accessorKey: 'name', header: t('adminCoupons.name') }, { id: 'kind', header: t('adminCoupons.kind') }, { id: 'usage', header: t('adminWorkspace.count') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }])
const kindItems = computed(() => [
  { value: 'purchase_once', label: t('adminCoupons.oneTime') },
  { value: 'purchase_recurring', label: t('adminCoupons.recurring') },
  { value: 'balance_add', label: t('adminCoupons.balanceAdd') },
  { value: 'balance_multiply', label: t('adminCoupons.balanceMultiply') },
])
const discountItems = computed(() => [
  { value: 'percent', label: t('adminCoupons.percent') },
  { value: 'fixed', label: t('adminCoupons.fixed') },
])
function kindLabel(kind: CouponDefinition['kind']): string {
  return t('adminCoupons.' + (kind === 'purchase_once' ? 'oneTime' : kind === 'purchase_recurring' ? 'recurring' : kind === 'balance_add' ? 'balanceAdd' : 'balanceMultiply'))
}
</script>

<template>
  <section class="admin-panel">
    <div class="admin-panel__heading"><div><h2>{{ t('adminCoupons.title') }}</h2><p>{{ t('adminCoupons.copy') }}</p></div><div class="row-actions"><UButton color="neutral" variant="outline" icon="i-ph-arrow-clockwise" :label="t('common.refresh')" :loading="refreshing" :disabled="busy || refreshing" @click="load" /><UButton icon="i-ph-plus" :label="t('adminCoupons.new')" :disabled="busy || loading" @click="edit()" /></div></div>
    <USlideover :open="editingId !== undefined" :title="editingId ? t('adminCoupons.edit') : t('adminCoupons.definition')" :description="t('adminCoupons.financialHint')" :dismissible="!busy" @update:open="!$event && close()">
      <template #body>
        <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        <form @submit.prevent="save">
          <fieldset class="admin-editor-form" :disabled="busy">
            <UFormField name="coupon-code" :label="t('adminCoupons.code')" required><UInput v-model.trim="draft.code" class="w-full" pattern="[A-Za-z0-9_\-]+" :maxlength="40" /></UFormField>
            <UFormField name="coupon-name" :label="t('adminCoupons.name')" required><UInput v-model.trim="draft.name" class="w-full" :maxlength="80" /></UFormField>
            <UFormField name="coupon-kind" :label="t('adminCoupons.kind')"><USelect v-model="draft.kind" class="w-full" :items="kindItems" :disabled="busy || Boolean(editingId)" /></UFormField>
            <UFormField v-if="draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring'" name="discount-mode" :label="t('adminCoupons.discountMode')"><USelect v-model="draft.discountMode" class="w-full" :items="discountItems" :disabled="busy || Boolean(editingId)" /></UFormField>
            <UFormField v-if="(draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring') && draft.discountMode === 'percent'" name="discount-percent" :label="t('adminCoupons.discountPercent')"><UInput v-model="draft.percent" class="w-full" type="number" :min="0.01" :max="100" :step="0.01" /></UFormField>
            <UFormField v-if="draft.kind === 'balance_multiply'" name="balance-multiplier" :label="t('adminCoupons.balanceMultiplier')"><UInput v-model="draft.factor" class="w-full" type="number" :min="1.0001" :max="100" :step="0.0001" /></UFormField>
            <TxbAmountField v-if="draft.kind === 'balance_add' || ((draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring') && draft.discountMode === 'fixed')" id="coupon-value" v-model="draft.valueTxb" :label="t('adminCoupons.txbValue')" min-minor="1" required />
            <TxbAmountField v-if="(draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring') && draft.discountMode === 'percent'" id="coupon-cap" v-model="draft.capTxb" :label="t('adminCoupons.cap')" :hint="t('adminCoupons.capHint')" />
            <div v-if="optionsError" class="catalog-editor__wide"><InlineNotice tone="warning">{{ optionsError }}</InlineNotice><UButton color="neutral" variant="outline" :label="t('common.tryAgain')" :loading="optionsLoading" :disabled="optionsLoading" @click="loadOptions" /></div>
            <UFormField name="combo-ids" :label="t('adminCoupons.comboIds')"><USelectMenu v-model="draft.eligibleComboIds" class="w-full" :items="comboItems" value-key="value" label-key="label" multiple :placeholder="t('adminCoupons.comboPlaceholder')" :disabled="busy || draft.kind === 'balance_add' || draft.kind === 'balance_multiply'" /></UFormField>
            <UFormField name="squad-ids" :label="t('adminCoupons.squadIds')"><USelectMenu v-model="draft.eligibleSquadIds" class="w-full" :items="squadItems" value-key="value" label-key="label" multiple :placeholder="t('adminCoupons.squadPlaceholder')" :disabled="busy || draft.kind === 'balance_add' || draft.kind === 'balance_multiply'" /></UFormField>
            <UFormField name="coupon-expiry" :label="t('adminCoupons.expires')"><UInput v-model="draft.expiresAt" class="w-full" type="date" /></UFormField>
            <UFormField name="global-limit" :label="t('adminCoupons.globalLimit')"><UInput v-model="draft.globalLimit" class="w-full" inputmode="numeric" pattern="[0-9]*" /></UFormField>
            <UFormField name="user-limit" :label="t('adminCoupons.userLimit')"><UInput v-model="draft.perUserLimit" class="w-full" inputmode="numeric" pattern="[0-9]*" /></UFormField>
            <SwitchField id="coupon-active" v-model="draft.active" :label="t('common.active')" />
            <div class="button-row"><UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="close" /><UButton type="submit" :loading="busy" :disabled="busy" :label="busy ? t('common.saving') : editingId ? t('adminCoupons.save') : t('adminCoupons.create')" /></div>
          </fieldset>
        </form>
      </template>
    </USlideover>
    <UAlert v-if="error" role="alert" class="admin-error" color="warning" variant="soft" icon="i-ph-warning" :description="error" />
    <UTable class="admin-table" :data="items" :columns="columns" :loading="loading">
      <template #loading><div class="admin-loading" role="status"><USkeleton class="h-10 w-full" /><span>{{ t('common.loading') }}</span></div></template>
      <template #kind-cell="{ row }">{{ kindLabel(row.original.kind) }}</template>
      <template #usage-cell="{ row }">{{ row.original.usageCount }} / {{ row.original.globalUseLimit ?? t('adminCoupons.unlimited') }}</template>
      <template #status-cell="{ row }"><UBadge :color="row.original.active ? 'success' : 'neutral'" variant="subtle" :label="t(row.original.active ? 'common.active' : 'adminCatalog.paused')" /></template>
      <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminCoupons.editNamed', { code: row.original.code })" :disabled="busy || loading" @click="edit(row.original)" /><UButton v-if="row.original.active" color="error" variant="ghost" icon="i-ph-pause" :disabled="busy" :label="t('adminCoupons.deactivate')" @click="requestDeactivation(row.original)" /></div></template>
      <template #empty>{{ t('adminCoupons.none') }}</template>
    </UTable>
    <ConfirmDialog :open="Boolean(deactivating)" :title="t('adminCoupons.deactivate')" :description="t('adminCoupons.deactivateConfirm', { code: deactivating?.code ?? '' })" :confirm-label="t('adminCoupons.deactivate')" :busy="busy" :error="error" danger @update:open="!$event && closeDeactivation()" @confirm="deactivate" />
  </section>
</template>

<style scoped>
.admin-error { margin: 1rem; }
.admin-editor-form { border: 0; padding: 0; min-width: 0; }
</style>
