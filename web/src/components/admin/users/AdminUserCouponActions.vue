<script setup lang="ts">
import { computed, shallowRef } from 'vue'

import { adminOperationsApi, type AdminUserDetail } from '@/api/adminOperations'
import { featuresApi, type CouponDefinition } from '@/api/features'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ userId: string; grants: AdminUserDetail['couponWallet'] }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
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
  error.value = null; reason.value = ''; selectedCouponID.value = ''
  try { coupons.value = (await featuresApi.getAdminCoupons()).items } catch { error.value = t('adminUserProfile.actionFailed') }
  grantOpen.value = true
}

async function grant(): Promise<void> {
  if (!selectedCouponID.value || reason.value.length < 4 || busy.value) return
  busy.value = true
  try {
    await adminOperationsApi.grantUserCoupon(props.userId, selectedCouponID.value, reason.value, createUuid())
    grantOpen.value = false; emit('changed');
  } catch { error.value = t('adminUserProfile.actionFailed'); } finally { busy.value = false }
}

async function discard(): Promise<void> {
  if (!discarding.value || busy.value) return
  busy.value = true
  try {
    await adminOperationsApi.discardUserCoupon(props.userId, discarding.value.id, createUuid())
    discarding.value = null; emit('changed');
  } catch { error.value = t('adminUserProfile.actionFailed'); } finally { busy.value = false }
}
</script>

<template>
  <section class="admin-section">
    <div class="admin-profile-section__heading"><div><h3>{{ t('adminUserProfile.couponWallet') }}</h3><p>{{ t('adminUserProfile.addCouponHint') }}</p></div><UButton color="neutral" variant="ghost" icon="i-ph-plus" :label="t('adminUserProfile.addCoupon')" @click="openGrant" /></div>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <UTable class="admin-table" :data="grants" :columns="[{ id: 'coupon', header: t('adminWorkspace.code') }, { id: 'date', header: t('adminWorkspace.created') }, { id: 'actions', header: t('adminWorkspace.actions') }]">
      <template #coupon-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.coupon.name }}</strong><code>{{ row.original.coupon.code }}</code></div></template>
      <template #date-cell="{ row }">{{ formatDateTime(row.original.createdAt) }}</template>
      <template #actions-cell="{ row }"><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminUserProfile.removeCoupon')" :disabled="busy" @click="discarding = row.original" /></template>
      <template #empty>{{ t('adminUserProfile.noCoupons') }}</template>
    </UTable>
    <UModal v-model:open="grantOpen" :title="t('adminUserProfile.addCoupon')" :description="t('adminUserProfile.addCouponHint')" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }"><template #body><InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice><div class="form-stack"><UFormField name="coupon" :label="t('adminUserProfile.couponWallet')"><USelectMenu v-model="selectedCouponID" :items="couponItems" value-key="value" label-key="label" /></UFormField><UFormField name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="reason" :rows="3" :minlength="4" :maxlength="300" /></UFormField></div></template><template #footer="{ close }"><UButton color="neutral" variant="outline" :label="t('common.cancel')" @click="close" /><UButton color="primary" :label="t('common.confirm')" :loading="busy" :disabled="!selectedCouponID || reason.length < 4" @click="grant" /></template></UModal>
    <UModal :open="discarding !== null" :title="t('adminUserProfile.removeCoupon')" :description="t('adminUserProfile.removeCouponHint')" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }" @update:open="!$event && (discarding = null)"><template #body><InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice></template><template #footer="{ close }"><UButton color="neutral" variant="outline" :label="t('common.cancel')" @click="close" /><UButton color="error" :label="t('adminUserProfile.removeCoupon')" :loading="busy" @click="discard" /></template></UModal>
  </section>
</template>
