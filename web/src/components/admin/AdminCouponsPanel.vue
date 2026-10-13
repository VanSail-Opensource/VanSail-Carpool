<script setup lang="ts">
import { restoreItems } from '@/api/cache/restore'
import { computed, onMounted, reactive, shallowRef } from 'vue'

import { api } from '@/api/client'
import { featuresApi, type CouponDefinition } from '@/api/features'
import type { Combo, SquadProduct } from '@/api/types'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { localizedError, useI18n } from '@/i18n'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'

const items = shallowRef<CouponDefinition[]>([])
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const combos = shallowRef<Combo[]>([])
const squads = shallowRef<SquadProduct[]>([])
const editingId = shallowRef<string | null | undefined>(undefined)
const deactivating = shallowRef<CouponDefinition | null>(null)
const draft = reactive({
  code: '', name: '', kind: 'purchase_once' as CouponDefinition['kind'], discountMode: 'percent' as 'fixed' | 'percent',
  valueTxb: '5.00', percent: 10, factor: 2, capTxb: '', globalLimit: '', perUserLimit: '1',
  eligibleComboIds: [] as string[], eligibleSquadIds: [] as string[], expiresAt: '', active: true,
})
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
const comboItems = computed(() => combos.value.map((combo) => ({ value: combo.id, label: combo.name })))
const squadItems = computed(() => squads.value.map((squad) => ({ value: squad.id, label: squad.name })))

async function load(): Promise<void> {
  loading.value = !restoreItems('/api/v1/admin/coupons', items)
  error.value = null
  try { items.value = (await featuresApi.getAdminCoupons()).items }
  catch (caught) { error.value = localizedError(caught, 'adminCoupons.loadFailed') }
  finally { loading.value = false }
}

async function loadOptions(): Promise<void> {
  restoreItems('/api/v1/admin/combos', combos)
  restoreItems('/api/v1/admin/squad-products', squads)
  try {
    const [comboResponse, squadResponse] = await Promise.all([
      api.getAdminResource<{ items: Combo[] }>('combos'),
      api.getAdminResource<{ items: SquadProduct[] }>('squad-products'),
    ])
    combos.value = comboResponse.items
    squads.value = squadResponse.items
  } catch { /* Coupon editing remains usable when option data is temporarily unavailable. */ }
}

function edit(coupon?: CouponDefinition): void {
  editingId.value = coupon?.id ?? null
  Object.assign(draft, coupon ? {
    code: coupon.code, name: coupon.name, kind: coupon.kind, discountMode: coupon.discountMode ?? 'fixed',
    valueTxb: ['purchase_once', 'purchase_recurring'].includes(coupon.kind) && coupon.discountMode === 'percent' || coupon.kind === 'balance_multiply' ? '5.00' : txbInputFromMinor(coupon.valueMinorOrBps),
    percent: coupon.discountMode === 'percent' ? Number(coupon.valueMinorOrBps) / 100 : 10,
    factor: coupon.kind === 'balance_multiply' ? Number(coupon.valueMinorOrBps) / 10000 : 2,
    capTxb: coupon.percentCapMinor ? txbInputFromMinor(coupon.percentCapMinor) : '',
    globalLimit: coupon.globalUseLimit === null ? '' : String(coupon.globalUseLimit),
    perUserLimit: coupon.perUserUseLimit === null ? '' : String(coupon.perUserUseLimit),
    eligibleComboIds: [...coupon.eligibleComboIds], eligibleSquadIds: [...coupon.eligibleSquadIds],
    expiresAt: coupon.expiresAt?.slice(0, 10) ?? '', active: coupon.active,
  } : {
    code: '', name: '', kind: 'purchase_once', discountMode: 'percent', valueTxb: '5.00', percent: 10,
    factor: 2, capTxb: '', globalLimit: '', perUserLimit: '1', eligibleComboIds: [], eligibleSquadIds: [], expiresAt: '', active: true,
  })
}

async function save(): Promise<void> {
  busy.value = true
  error.value = null
  try {
    const purchase = draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring'
    const percentage = purchase && draft.discountMode === 'percent'
    const valueMinorOrBps = percentage ? String(Math.round(draft.percent * 100))
      : draft.kind === 'balance_multiply' ? String(Math.round(draft.factor * 10000)) : moneyFromTxbInput(draft.valueTxb)
    if (!valueMinorOrBps) return
    await featuresApi.saveAdminCoupon(editingId.value ?? null, {
      code: draft.code.trim().toUpperCase(), name: draft.name, kind: draft.kind,
      discountMode: purchase ? draft.discountMode : undefined, valueMinorOrBps,
      percentCapMinor: percentage && draft.capTxb ? moneyFromTxbInput(draft.capTxb) : null,
      eligibleComboIds: purchase ? [...draft.eligibleComboIds] : [],
      eligibleSquadIds: purchase ? [...draft.eligibleSquadIds] : [],
      expiresAt: draft.expiresAt ? new Date(`${draft.expiresAt}T23:59:59Z`).toISOString() : null,
      active: draft.active, globalUseLimit: draft.globalLimit ? Number(draft.globalLimit) : null,
      perUserUseLimit: draft.perUserLimit ? Number(draft.perUserLimit) : null,
    })
    editingId.value = undefined
    await load()
  } catch (caught) { error.value = localizedError(caught, 'adminCoupons.saveFailed') }
  finally { busy.value = false }
}

async function deactivate(): Promise<void> {
  if (!deactivating.value) return
  busy.value = true
  error.value = null
  try {
    await featuresApi.deactivateAdminCoupon(deactivating.value.id)
    deactivating.value = null
    await load()
  } catch (caught) { error.value = localizedError(caught, 'adminCoupons.deactivateFailed') }
  finally { busy.value = false }
}

function kindLabel(kind: CouponDefinition['kind']): string {
  return t(`adminCoupons.${kind === 'purchase_once' ? 'oneTime' : kind === 'purchase_recurring' ? 'recurring' : kind === 'balance_add' ? 'balanceAdd' : 'balanceMultiply'}`)
}

onMounted(() => { void load(); void loadOptions() })
</script>

<template>
  <section class="admin-panel">
    <div class="admin-panel__heading"><div><h2>{{ t('adminCoupons.title') }}</h2><p>{{ t('adminCoupons.copy') }}</p></div><UButton icon="i-ph-plus" :label="t('adminCoupons.new')" @click="edit()" /></div>
    <USlideover :open="editingId !== undefined" :title="editingId ? t('adminCoupons.edit') : t('adminCoupons.definition')" :description="t('adminCoupons.financialHint')" :dismissible="!busy" @update:open="!$event && (editingId = undefined)">
      <template #body>
        <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        <form class="admin-editor-form" @submit.prevent="save">
          <UFormField name="coupon-code" :label="t('adminCoupons.code')" required><UInput v-model.trim="draft.code" class="w-full" pattern="[A-Za-z0-9_\-]+" :maxlength="40" /></UFormField>
          <UFormField name="coupon-name" :label="t('adminCoupons.name')" required><UInput v-model.trim="draft.name" class="w-full" :maxlength="80" /></UFormField>
          <UFormField name="coupon-kind" :label="t('adminCoupons.kind')" :hint="editingId ? t('adminCoupons.financialHint') : undefined"><USelect v-model="draft.kind" class="w-full" :items="kindItems" :disabled="Boolean(editingId)" /></UFormField>
          <UFormField v-if="draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring'" name="discount-mode" :label="t('adminCoupons.discountMode')"><USelect v-model="draft.discountMode" class="w-full" :items="discountItems" :disabled="Boolean(editingId)" /></UFormField>
          <UFormField v-if="(draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring') && draft.discountMode === 'percent'" name="discount-percent" :label="t('adminCoupons.discountPercent')"><UInput v-model.number="draft.percent" class="w-full" type="number" :min="0.01" :max="100" :step="0.01" /></UFormField>
          <UFormField v-if="draft.kind === 'balance_multiply'" name="balance-multiplier" :label="t('adminCoupons.balanceMultiplier')"><UInput v-model.number="draft.factor" class="w-full" type="number" :min="1.01" :max="100" :step="0.01" /></UFormField>
          <TxbAmountField v-if="draft.kind === 'balance_add' || ((draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring') && draft.discountMode === 'fixed')" id="coupon-value" v-model="draft.valueTxb" :label="t('adminCoupons.txbValue')" min-minor="1" required />
          <TxbAmountField v-if="(draft.kind === 'purchase_once' || draft.kind === 'purchase_recurring') && draft.discountMode === 'percent'" id="coupon-cap" v-model="draft.capTxb" :label="t('adminCoupons.cap')" :hint="t('adminCoupons.capHint')" />
          <UFormField name="combo-ids" :label="t('adminCoupons.comboIds')"><USelectMenu v-model="draft.eligibleComboIds" class="w-full" :items="comboItems" value-key="value" label-key="label" multiple :placeholder="t('adminCoupons.comboPlaceholder')" :disabled="draft.kind === 'balance_add' || draft.kind === 'balance_multiply'" /></UFormField>
          <UFormField name="squad-ids" :label="t('adminCoupons.squadIds')"><USelectMenu v-model="draft.eligibleSquadIds" class="w-full" :items="squadItems" value-key="value" label-key="label" multiple :placeholder="t('adminCoupons.squadPlaceholder')" :disabled="draft.kind === 'balance_add' || draft.kind === 'balance_multiply'" /></UFormField>
          <UFormField name="coupon-expiry" :label="t('adminCoupons.expires')"><UInput v-model="draft.expiresAt" class="w-full" type="date" /></UFormField>
          <UFormField name="global-limit" :label="t('adminCoupons.globalLimit')"><UInput v-model="draft.globalLimit" class="w-full" inputmode="numeric" pattern="[0-9]*" /></UFormField>
          <UFormField name="user-limit" :label="t('adminCoupons.userLimit')"><UInput v-model="draft.perUserLimit" class="w-full" inputmode="numeric" pattern="[0-9]*" /></UFormField>
          <SwitchField id="coupon-active" v-model="draft.active" :label="t('common.active')" />
          <div class="button-row"><UButton color="neutral" variant="outline" :label="t('common.cancel')" @click="editingId = undefined" /><UButton type="submit" :loading="busy" :disabled="busy" :label="busy ? t('common.saving') : editingId ? t('adminCoupons.save') : t('adminCoupons.create')" /></div>
        </form>
      </template>
    </USlideover>
    <UAlert v-if="error" class="admin-error" color="warning" variant="soft" icon="i-ph-warning" :description="error" />
    <UTable class="admin-table" :data="items" :columns="columns" :loading="loading">
      <template #loading><div class="admin-loading" role="status"><USkeleton class="h-10 w-full" /><span>{{ t('common.loading') }}</span></div></template>
      <template #kind-cell="{ row }">{{ kindLabel(row.original.kind) }}</template>
      <template #usage-cell="{ row }">{{ row.original.usageCount }} / {{ row.original.globalUseLimit ?? t('adminCoupons.unlimited') }}</template>
      <template #status-cell="{ row }"><UBadge :color="row.original.active ? 'success' : 'neutral'" variant="subtle" :label="t(row.original.active ? 'common.active' : 'adminCatalog.paused')" /></template>
      <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminCoupons.editNamed', { code: row.original.code })" @click="edit(row.original)" /><UButton v-if="row.original.active" color="error" variant="ghost" icon="i-ph-pause" :disabled="busy" :label="t('adminCoupons.deactivate')" @click="deactivating = row.original" /></div></template>
      <template #empty>{{ t('adminCoupons.none') }}</template>
    </UTable>
    <ConfirmDialog :open="Boolean(deactivating)" :title="t('adminCoupons.deactivate')" :description="t('adminCoupons.deactivateConfirm', { code: deactivating?.code ?? '' })" :confirm-label="t('adminCoupons.deactivate')" :busy="busy" danger @update:open="!$event && (deactivating = null)" @confirm="deactivate" />
  </section>
</template>

<style scoped>.admin-error { margin: 1rem; }</style>
