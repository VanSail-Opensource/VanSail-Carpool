<script setup lang="ts">
import { computed, shallowRef, toRef, watch } from 'vue'

import UserPricingAnalysis from '../pricing/UserPricingAnalysis.vue'
import { usdtApi } from '@/api/usdt'
import AdminPaymentCreditDialog from './AdminPaymentCreditDialog.vue'
import { adminOperationsApi, type AdminEntitlement, type AdminEntitlementRefundRequest, type AdminUserDetail, type CourtesyCredit, type EntitlementEditRequest, type OperationReceipt, type OperationResolution } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import ConnectionUnblockDialog from '@/components/connections/ConnectionUnblockDialog.vue'
import { useOperationReceipt } from '@/composables/useOperationReceipt'
import { useI18n } from '@/i18n'
import { formatMoney } from '@/utils/format'
import AdminReasonDialog from '../AdminReasonDialog.vue'
import AdminEntitlementEditor from './AdminEntitlementEditor.vue'
import AdminEntitlementRefundDialog from './AdminEntitlementRefundDialog.vue'
import AdminOperationResolutionDialog from './AdminOperationResolutionDialog.vue'
import AdminUserEntitlements from './AdminUserEntitlements.vue'
import AdminUserAccountContext from './AdminUserAccountContext.vue'
import AdminUserCouponActions from './AdminUserCouponActions.vue'
import AdminUserMetrics from './AdminUserMetrics.vue'
import AdminUserBalanceAdjustment from './AdminUserBalanceAdjustment.vue'
import AdminUserHistory from './AdminUserHistory.vue'
import AdminUserIPBlocks from './AdminUserIPBlocks.vue'
import AdminUserOverview from './AdminUserOverview.vue'
import AdminUserPM from './AdminUserPM.vue'
import AdminUserProviderActions from './AdminUserProviderActions.vue'
import { useAdminUserProfile } from './useAdminUserProfile'

const props = defineProps<{ userId: string }>()
const emit = defineEmits<{ back: [] }>()
const { t } = useI18n()
const profile = useAdminUserProfile(toRef(props, 'userId'))
const tabs = computed(() => [
  { value: 'entitlements', label: t('adminUserProfile.entitlements') },
  { value: 'pricing', label: t('userPrices.title') },
  { value: 'metrics', label: t('adminNative.title') },
  { value: 'history', label: t('adminUserProfile.records') },
  { value: 'account', label: t('adminUserProfile.actionsAndContext') },
])
const editing = shallowRef<AdminEntitlement | null>(null)
const refunding = shallowRef<AdminEntitlement | null>(null)
const refundingPayment = shallowRef<AdminUserDetail['payments'][number] | null>(null)
const creditingPayment = shallowRef<AdminUserDetail['payments'][number] | null>(null)
const paymentReason = shallowRef('')
const courtesyMessage = shallowRef<string | null>(null)
const queuedOperation = shallowRef<OperationReceipt | null>(null)
const resolving = shallowRef<OperationReceipt | null>(null)
const unblockingIP = shallowRef<AdminUserDetail['ipBlocks'][number] | null>(null)
const unblockOperation = useOperationReceipt()
const busy = computed(() => profile.busyAction.value !== null)
const dialogError = computed(() => profile.optionsError.value ?? profile.error.value)

async function openEditor(item: AdminEntitlement): Promise<void> {
  profile.clearActionError()
  editing.value = item
  await profile.loadOptions()
}

function openRefund(item: AdminEntitlement): void {
  profile.clearActionError()
  refunding.value = item
}

function openPaymentRefund(payment: AdminUserDetail['payments'][number]): void {
  profile.clearActionError()
  paymentReason.value = ''
  refundingPayment.value = payment
}

function openPaymentCredit(payment: AdminUserDetail['payments'][number]): void {
  profile.clearActionError()
  paymentReason.value = ''
  creditingPayment.value = payment
}

async function saveEntitlement(body: EntitlementEditRequest): Promise<void> {
  const item = editing.value
  if (!item) return
  const ok = await profile.perform(`edit:${item.id}`, (key) =>
    adminOperationsApi.editEntitlement(props.userId, item.id, body, key))
  if (ok) editing.value = null
}

async function refundEntitlement(body: AdminEntitlementRefundRequest): Promise<void> {
  const item = refunding.value
  if (!item) return
  let receipt: OperationReceipt | null = null
  const ok = await profile.perform(`refund:${item.id}`, async (key) => {
    receipt = await adminOperationsApi.refundEntitlement(props.userId, item.id, body, key)
    return receipt
  })
  if (ok) queuedOperation.value = receipt
  if (ok) refunding.value = null
}

function queueOperation(receipt: OperationReceipt): void {
  queuedOperation.value = receipt
}

async function refundPayment(): Promise<void> {
  const payment = refundingPayment.value
  if (!payment) return
  const ok = await profile.perform(`refund-payment:${payment.id}`, (key) =>
    adminOperationsApi.refundPayment(payment.id, paymentReason.value, key))
  if (ok) refundingPayment.value = null
}

async function creditPayment(transferId: string, reason: string): Promise<void> {
  const payment = creditingPayment.value
  if (!payment) return
  if (transferId) {
    const ok = await profile.perform(`settle-payment:${payment.id}`, () => usdtApi.resolve(transferId, payment.id, reason))
    if (ok) { creditingPayment.value = null; courtesyMessage.value = t('adminUsdt.settled') }
    return
  }
  const credit = shallowRef<CourtesyCredit | null>(null)
  const ok = await profile.perform(`credit-payment:${payment.id}`, async () => {
    credit.value = await adminOperationsApi.creditPayment(payment.id, reason)
    return credit.value
  })
  const completedCredit = credit.value
  if (!ok || !completedCredit) return
  courtesyMessage.value = completedCredit.replayed
    ? t('adminUserProfile.courtesyCreditAlreadyRecorded')
    : t('adminUserProfile.courtesyCreditSuccess', { amount: formatMoney(completedCredit.txb) })
  creditingPayment.value = null
}

async function resolveOperation(payload: { resolution: OperationResolution; reason: string }): Promise<void> {
  const operation = resolving.value
  if (!operation) return
  const ok = await profile.perform(`resolve:${operation.id}`, (key) =>
    adminOperationsApi.resolveOperation(operation.id, payload.resolution, payload.reason, key))
  if (ok) resolving.value = null
}

function refresh(): void {
  profile.clearActionError()
  void profile.load()
}

function openIPUnblock(block: AdminUserDetail['ipBlocks'][number]): void {
  profile.clearActionError()
  unblockOperation.reset()
  unblockingIP.value = block
}

async function unblockIP(): Promise<void> {
  const block = unblockingIP.value
  if (!block) return
  await profile.perform(`unblock-ip:${block.id}`, async (key) => {
    const receipt = await adminOperationsApi.unblockIP(props.userId, block.id, key)
    unblockOperation.track(receipt)
    return receipt
  })
}

watch(() => unblockOperation.receipt.value, (receipt) => {
  if (!receipt || !unblockOperation.terminal.value) return
  void profile.load()
  if (receipt.status === 'succeeded') unblockingIP.value = null
})
</script>

<template>
  <section class="admin-panel admin-profile">
    <div class="admin-profile-toolbar">
      <UButton color="neutral" variant="outline" :to="`/admin/users/${encodeURIComponent(userId)}/pricing`" :label="t('pricing.title')" />
      <UButton color="neutral" variant="ghost" icon="i-ph-arrow-left" :label="t('adminUserProfile.backToUsers')" @click="emit('back')" />
      <UButton color="neutral" variant="outline" icon="i-ph-arrow-clockwise" :label="t('common.refresh')" :loading="profile.loading.value" @click="refresh" />
    </div>
    <InlineNotice v-if="profile.conflict.value" tone="warning" :title="t('adminUserProfile.conflictTitle')">{{ t('adminUserProfile.conflict') }}</InlineNotice>
    <InlineNotice v-else-if="profile.error.value" tone="warning">{{ profile.error.value }}</InlineNotice>
    <InlineNotice v-if="courtesyMessage" tone="success">{{ courtesyMessage }}</InlineNotice>
    <InlineNotice v-if="queuedOperation" tone="success">{{ t('adminUserProfile.operationQueued', { id: queuedOperation.id }) }}</InlineNotice>
    <div v-if="profile.loading.value" class="admin-loading" :aria-label="t('adminUserProfile.loading')">
      <USkeleton class="h-24" /><USkeleton class="h-48" /><USkeleton class="h-36" />
    </div>
    <template v-else-if="profile.detail.value">
      <AdminUserOverview :detail="profile.detail.value" />
      <AdminUserBalanceAdjustment :user-id="userId" @changed="refresh" />
      <nav class="row-actions" :aria-label="t('pricing.navigation')"><UButton v-for="tab in tabs" :key="tab.value" :href="`#user-${tab.value}`" color="neutral" variant="ghost" :label="tab.label" /></nav>
      <UserPricingAnalysis id="user-pricing" :key="userId" :user-id="userId" />
      <AdminUserEntitlements id="user-entitlements" :items="profile.detail.value.entitlements" :busy="busy" @edit="openEditor" @refund="openRefund" />
      <AdminUserMetrics id="user-metrics" :user-id="userId" />
      <AdminUserHistory id="user-history" :detail="profile.detail.value" :busy="busy" @resolve="resolving = $event" @refund-payment="openPaymentRefund" @credit-payment="openPaymentCredit" />
      <div id="user-account" class="admin-workspace-grid">
        <AdminUserProviderActions :user-id="userId" :detail="profile.detail.value" @changed="refresh" @queued="queueOperation" />
        <AdminUserCouponActions :user-id="userId" :grants="profile.detail.value.couponWallet" @changed="refresh" />
        <AdminUserAccountContext :detail="profile.detail.value" />
        <AdminUserPM :conversation="profile.detail.value.pmConversation" @changed="refresh" />
        <AdminUserIPBlocks :items="profile.detail.value.ipBlocks" :busy="busy" @unblock="openIPUnblock" />
      </div>
    </template>
    <div v-else class="empty-inline"><div><h3>{{ t('adminUserProfile.unavailable') }}</h3><p>{{ t('adminUserProfile.unavailableHint') }}</p></div></div>

    <AdminEntitlementEditor :open="editing !== null" :item="editing" :combos="profile.options.value.combos" :squads="profile.options.value.squads" :busy="busy" :options-loading="profile.optionsLoading.value" :error="dialogError" @update:open="!$event && (editing = null)" @save="saveEntitlement" />
    <AdminEntitlementRefundDialog :open="refunding !== null" :item="refunding" :busy="busy" :error="profile.error.value" @update:open="!$event && (refunding = null)" @refund="refundEntitlement" />
    <AdminReasonDialog :open="refundingPayment !== null" v-model:reason="paymentReason" :title="t('adminUserProfile.refundPaymentTitle')" :description="t('adminUserProfile.refundPaymentHint')" :confirm-label="t('adminUserProfile.issueRefundPayment')" :busy="busy" :error="profile.error.value" danger @update:open="!$event && (refundingPayment = null)" @confirm="refundPayment" />
    <AdminPaymentCreditDialog :payment="creditingPayment" :busy="busy" :error="profile.error.value" @close="creditingPayment = null" @confirm="creditPayment" />
    <AdminOperationResolutionDialog :open="resolving !== null" :operation="resolving" :busy="busy" :error="profile.error.value" @update:open="!$event && (resolving = null)" @resolve="resolveOperation" />
    <ConnectionUnblockDialog :open="unblockingIP !== null" :block="unblockingIP" :receipt="unblockOperation.receipt.value" :busy="profile.busyAction.value?.startsWith('unblock-ip:')" :checking="unblockOperation.checking.value" :error="profile.error.value ?? unblockOperation.error.value" @update:open="!$event && (unblockingIP = null)" @confirm="unblockIP" @refresh="unblockOperation.refresh" />
  </section>
</template>
