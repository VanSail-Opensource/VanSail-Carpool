<script setup lang="ts">
import { computed, reactive, shallowRef } from 'vue'

import { adminOperationsApi, type AdminUserDetail, type OperationReceipt } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ userId: string; detail: AdminUserDetail }>()
const emit = defineEmits<{ changed: []; queued: [receipt: OperationReceipt] }>()
const { t } = useI18n()
const panel = shallowRef<'ban' | 'unban' | null>(null)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const form = reactive({ durationMinutes: 1440, reason: '' })
const hasBan = computed(() => props.detail.temporaryBan !== null)

function open(next: 'ban' | 'unban'): void {
  error.value = null
  form.reason = ''
  form.durationMinutes = 1440
  panel.value = next
}

async function submit(): Promise<void> {
  if (busy.value || form.reason.length < 4 || !panel.value) return
  busy.value = true
  error.value = null
  try {
    const key = createUuid()
    let receipt: OperationReceipt
    if (panel.value === 'ban') receipt = await adminOperationsApi.temporaryBan(props.userId, form.durationMinutes, form.reason, key)
    else receipt = await adminOperationsApi.temporaryUnban(props.userId, form.reason, key)
    panel.value = null
    emit('queued', receipt)
    emit('changed')
  } catch {
    error.value = t('adminUserProfile.actionFailed')
  } finally { busy.value = false }
}

</script>

<template>
  <section class="admin-profile-section">
    <div class="admin-profile-section__heading"><div><h3>{{ t('adminUserProfile.connectionControls') }}</h3><p>{{ t('adminUserProfile.connectionControlsHint') }}</p></div></div>
    <div class="row-actions">
      <UButton v-if="!hasBan" color="error" variant="outline" icon="i-ph-prohibit" :disabled="busy" :label="t('adminUserProfile.temporaryBan')" @click="open('ban')" />
      <UButton v-else color="primary" variant="outline" icon="i-ph-check-circle" :disabled="busy" :label="t('adminUserProfile.temporaryUnban')" @click="open('unban')" />
    </div>
    <p v-if="detail.temporaryBan" class="admin-profile-empty">{{ t('adminUserProfile.temporaryBanUntil', { date: formatDateTime(detail.temporaryBan.expiresAt) }) }}</p>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <USlideover :open="panel !== null" :title="panel === 'ban' ? t('adminUserProfile.temporaryBan') : t('adminUserProfile.temporaryUnban')" :dismissible="!busy" @update:open="!$event && (panel = null)">
      <template #body><InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice><div class="form-stack"><UFormField v-if="panel === 'ban'" name="duration" :label="t('adminUserProfile.banDuration')"><UInputNumber v-model="form.durationMinutes" :min="1" :max="525600" :step="1" class="w-full" /></UFormField><UFormField name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="form.reason" :rows="3" :minlength="4" :maxlength="300" /></UFormField></div></template>
      <template #footer><UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="panel = null" /><UButton :color="panel === 'ban' ? 'error' : 'primary'" :label="t('common.confirm')" :loading="busy" :disabled="busy || form.reason.length < 4" @click="submit" /></template>
    </USlideover>
  </section>
</template>
