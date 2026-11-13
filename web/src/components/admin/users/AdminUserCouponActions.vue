<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'

import { adminOperationsApi, type AdminUserDetail } from '@/api/adminOperations'
import { featuresApi, type CouponDefinition } from '@/api/features'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { createLatestRequest } from '@/utils/latestRequest'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ userId: string; grants: AdminUserDetail['couponWallet'] }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const latest = createLatestRequest()
const attempts = new Map<string, string>()
const coupons = shallowRef<CouponDefinition[]>([])
const grantOpen = shallowRef(false)
const selectedCouponID = shallowRef('')
const reason = shallowRef('')
const discarding = shallowRef<AdminUserDetail['couponWallet'][number] | null>(null)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const couponItems = computed(() => coupons.value
  .filter((coupon) => coupon.active && (coupon.kind === 'purchase_once' || coupon.kind === 'purchase_recurring'))
  .map((coupon) => ({ label: `${coupon.code} / ${coupon.name}`, value: coupon.id })))

async function openGrant(): Promise<void> {
  if (busy.value) return
  const ticket = latest.begin()
  error.value = null; reason.value = ''; selectedCouponID.value = ''
  try {
    const response = await featuresApi.getAdminCoupons()
    if (!latest.isCurrent(ticket)) return
    coupons.value = response.items
  } catch { if (latest.isCurrent(ticket)) error.value = t('adminUserProfile.actionFailed') }
  if (latest.isCurrent(ticket)) grantOpen.value = true
}

async function perform(fingerprint: string, task: (key: string) => Promise<unknown>): Promise<boolean> {
  const ticket = latest.begin()
  const key = attempts.get(fingerprint) ?? createUuid()
  attempts.set(fingerprint, key)
  busy.value = true; error.value = null
  try {
    await task(key)
    attempts.delete(fingerprint)
    if (!latest.isCurrent(ticket)) return false
    emit('changed')
    return true
  } catch {
    if (latest.isCurrent(ticket)) error.value = t('adminUserProfile.actionFailed')
    return false
  } finally { if (latest.isCurrent(ticket)) busy.value = false }
}

async function grant(): Promise<void> {
  if (!selectedCouponID.value || reason.value.trim().length < 4 || busy.value) return
  const userId = props.userId, couponId = selectedCouponID.value, note = reason.value.trim()
  if (await perform(JSON.stringify(['grant', userId, couponId, note]),
    key => adminOperationsApi.grantUserCoupon(userId, couponId, note, key))) grantOpen.value = false
}

async function discard(): Promise<void> {
  if (!discarding.value || busy.value) return
  const userId = props.userId, grantId = discarding.value.id
  if (await perform(JSON.stringify(['discard', userId, grantId]),
    key => adminOperationsApi.discardUserCoupon(userId, grantId, key))) discarding.value = null
}

watch(() => props.userId, () => {
  latest.invalidate(); busy.value = false; grantOpen.value = false; discarding.value = null
  selectedCouponID.value = ''; reason.value = ''; error.value = null
})
onScopeDispose(() => { latest.dispose(); attempts.clear() })

</script>

<template>
  <section class="admin-section">
    <div class="admin-profile-section__heading"><div><h3>{{ t('adminUserProfile.couponWallet') }}</h3><p>{{ t('adminUserProfile.addCouponHint') }}</p></div><UButton color="neutral" variant="ghost" icon="i-ph-plus" :label="t('adminUserProfile.addCoupon')" :disabled="busy" @click="openGrant" /></div>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <UTable class="admin-table" :data="grants" :columns="[{ id: 'coupon', header: t('adminWorkspace.code') }, { id: 'date', header: t('adminWorkspace.created') }, { id: 'actions', header: t('adminWorkspace.actions') }]">
      <template #coupon-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.coupon.name }}</strong><code>{{ row.original.coupon.code }}</code></div></template>
      <template #date-cell="{ row }">{{ formatDateTime(row.original.createdAt) }}</template>
      <template #actions-cell="{ row }"><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminUserProfile.removeCoupon')" :disabled="busy" @click="discarding = row.original" /></template>
      <template #empty>{{ t('adminUserProfile.noCoupons') }}</template>
    </UTable>
    <UModal v-model:open="grantOpen" :title="t('adminUserProfile.addCoupon')" :description="t('adminUserProfile.addCouponHint')" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }"><template #body><InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice><div class="form-stack"><UFormField name="coupon" :label="t('adminUserProfile.couponWallet')"><USelectMenu v-model="selectedCouponID" :disabled="busy" :items="couponItems" value-key="value" label-key="label" /></UFormField><UFormField name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="reason" :disabled="busy" :rows="3" :minlength="4" :maxlength="300" /></UFormField></div></template><template #footer="{ close }"><UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="close" /><UButton color="primary" :label="t('common.confirm')" :loading="busy" :disabled="busy || !selectedCouponID || reason.length < 4" @click="grant" /></template></UModal>
    <UModal :open="discarding !== null" :title="t('adminUserProfile.removeCoupon')" :description="t('adminUserProfile.removeCouponHint')" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }" @update:open="!$event && (discarding = null)"><template #body><InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice></template><template #footer="{ close }"><UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="close" /><UButton color="error" :label="t('adminUserProfile.removeCoupon')" :loading="busy" @click="discard" /></template></UModal>
  </section>
</template>
