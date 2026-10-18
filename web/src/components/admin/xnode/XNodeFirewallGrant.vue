<script setup lang="ts">
import { computed, reactive, shallowRef, onScopeDispose } from 'vue'
import { z } from 'zod'
import { firewallApi } from '@/api/firewall'
import { adminOperationsApi } from '@/api/adminOperations'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import { useOperationReceipt, operationIsActive } from '@/composables/useOperationReceipt'
import { localizedError, useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

const { t } = useI18n()
const draft = reactive({ prefix: '', hours: 24, reason: '' })
const submitting = shallowRef(false), error = shallowRef<string | null>(null)
const fingerprint = computed(() => JSON.stringify(draft))
const lastSubmitted = shallowRef('')
const keys = new Map<string, string>()
let active = true
onScopeDispose(() => { active = false; keys.clear() })
const tracker = useOperationReceipt(1500, adminOperationsApi.getOperation)
const { receipt, checking } = tracker
const blocked = computed(() => submitting.value || lastSubmitted.value === fingerprint.value && operationIsActive(receipt.value?.status))
const schema = computed(() => {
  const address = z.union([z.ipv4(), z.ipv6(), z.cidrv4(), z.cidrv6()])
  return z.object({
    prefix: z.string().trim().refine(value => address.safeParse(value).success, t('firewall.adminInvalidPrefix')),
    hours: z.number({ error: t('firewall.adminInvalidHours') }).int(t('firewall.adminInvalidHours')).min(1, t('firewall.adminInvalidHours')).max(8760, t('firewall.adminInvalidHours')),
    reason: z.string().max(1000, t('firewall.adminInvalidReason')),
  })
})
async function submit() {
  if (blocked.value) return
  const parsed = schema.value.safeParse({ ...draft })
  if (!parsed.success) return
  const body = parsed.data
  const fingerprint = JSON.stringify(draft)
  const key = keys.get(fingerprint) ?? createUuid()
  keys.set(fingerprint, key)
  submitting.value = true; error.value = null
  try {
    const result = await firewallApi.grant(body, key)
    if (!active) return
    lastSubmitted.value = fingerprint; keys.delete(fingerprint); tracker.track(result)
  } catch (caught) { if (active) error.value = localizedError(caught, 'firewall.saveFailed') }
  finally { if (active) submitting.value = false }
}
</script>

<template>
  <section class="grid gap-3 min-w-0">
    <div><h3 class="text-sm font-medium">{{ t('firewall.adminTitle') }}</h3><p class="text-xs text-muted">{{ t('firewall.adminHint') }}</p></div>
    <UForm :schema="schema" :state="draft" class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_9rem_auto] items-end" @submit="submit">
      <UFormField name="prefix" :label="t('firewall.adminPrefix')" required><UInput v-model="draft.prefix" :placeholder="t('firewall.adminPrefixPlaceholder')" class="w-full" :disabled="submitting" /></UFormField>
      <UFormField name="hours" :label="t('firewall.hours')" required><UInput v-model.number="draft.hours" type="number" min="1" max="8760" class="w-full" :disabled="submitting" /></UFormField>
      <UButton type="submit" icon="i-ph-lock-open" :label="t('firewall.adminSend')" :loading="submitting" :disabled="blocked" />
      <UFormField name="reason" :label="t('firewall.adminReason')" class="sm:col-span-3"><UInput v-model="draft.reason" :maxlength="1000" class="w-full" :disabled="submitting" /></UFormField>
    </UForm>
    <OperationStatusNotice :receipt="receipt" :error="error || tracker.error.value" :checking="checking" @refresh="tracker.refresh" />
  </section>
</template>
