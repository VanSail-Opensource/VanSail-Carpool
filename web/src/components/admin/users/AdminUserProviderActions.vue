<script setup lang="ts">
import { computed, onScopeDispose, reactive, shallowRef, watch } from 'vue'

import { adminOperationsApi, type AdminUserDetail, type OperationReceipt } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { createLatestRequest } from '@/utils/latestRequest'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ userId: string; detail: AdminUserDetail }>()
const emit = defineEmits<{ changed: []; queued: [receipt: OperationReceipt] }>()
const { t } = useI18n()
const latest = createLatestRequest()
const attempts = new Map<string, string>()
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
  if (busy.value || form.reason.trim().length < 4 || !panel.value) return
  const userId = props.userId, action = panel.value, duration = form.durationMinutes, reason = form.reason.trim()
  if (action === 'ban' && (!Number.isInteger(duration) || duration < 1 || duration > 525600)) return
  const fingerprint = JSON.stringify([userId, action, action === 'ban' ? duration : null, reason])
  const key = attempts.get(fingerprint) ?? createUuid()
  attempts.set(fingerprint, key)
  const ticket = latest.begin()
  busy.value = true
  error.value = null
  try {
    const receipt = action === 'ban'
      ? await adminOperationsApi.temporaryBan(userId, duration, reason, key)
      : await adminOperationsApi.temporaryUnban(userId, reason, key)
    attempts.delete(fingerprint)
    if (!latest.isCurrent(ticket)) return
    panel.value = null
    emit('queued', receipt)
    emit('changed')
  } catch {
    if (latest.isCurrent(ticket)) error.value = t('adminUserProfile.actionFailed')
  } finally { if (latest.isCurrent(ticket)) busy.value = false }
}

watch(() => props.userId, () => { latest.invalidate(); panel.value = null; busy.value = false; error.value = null })
onScopeDispose(() => { latest.dispose(); attempts.clear() })

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
      <template #body><InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice><div class="form-stack"><UFormField v-if="panel === 'ban'" name="duration" :label="t('adminUserProfile.banDuration')"><UInputNumber v-model="form.durationMinutes" :disabled="busy" :min="1" :max="525600" :step="1" class="w-full" /></UFormField><UFormField name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="form.reason" :disabled="busy" :rows="3" :minlength="4" :maxlength="300" /></UFormField></div></template>
      <template #footer><UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="panel = null" /><UButton :color="panel === 'ban' ? 'error' : 'primary'" :label="t('common.confirm')" :loading="busy" :disabled="busy || form.reason.length < 4" @click="submit" /></template>
    </USlideover>
  </section>
</template>
