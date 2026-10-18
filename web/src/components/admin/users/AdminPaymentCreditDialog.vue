<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import type { AdminUserDetail } from '@/api/adminOperations'
import { usdtApi, type USDTTransfer } from '@/api/usdt'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { localizedError, useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import { createLatestRequest } from '@/utils/latestRequest'

const props = defineProps<{ payment: AdminUserDetail['payments'][number] | null; busy: boolean; error: string | null }>()
const emit = defineEmits<{ close: []; confirm: [transferId: string, reason: string] }>()
const { t } = useI18n()
const selected = shallowRef('courtesy')
const reason = shallowRef('')
const evidence = shallowRef<USDTTransfer[]>([])
const loading = shallowRef(false)
const loadError = shallowRef<string | null>(null)
const latest = createLatestRequest()
let controller: globalThis.AbortController | undefined
const items = computed(() => [{ value: 'courtesy', label: t('adminUsdt.noEvidence') }, ...evidence.value.map(item => ({ value: item.id, label: `${item.amount} · ${formatDateTime(item.occurredAt)} · ${item.txHash}` }))])
const transfer = computed(() => evidence.value.find(item => item.id === selected.value))
async function load(): Promise<void> {
  controller?.abort()
  controller = new globalThis.AbortController()
  const generation = latest.begin()
  evidence.value = []; selected.value = 'courtesy'; loadError.value = null
  const payment = props.payment
  if (!payment || payment.provider !== 'builtin_usdt') { loading.value = false; return }
  loading.value = true
  try {
    const result = await usdtApi.review(undefined, controller.signal, payment.id)
    if (latest.isCurrent(generation)) evidence.value = result.items
  } catch (error) { if (latest.isCurrent(generation)) loadError.value = localizedError(error, 'errors.adminLoad') }
  finally { if (latest.isCurrent(generation)) loading.value = false }
}
watch(() => props.payment?.id, () => { reason.value = ''; void load() }, { immediate: true })
onScopeDispose(() => { controller?.abort(); latest.dispose(); evidence.value = [] })
</script>

<template>
  <UModal :open="payment !== null" :title="t('adminUserProfile.courtesyCreditTitle')" :description="t('adminUsdt.creditChoiceHint')" :dismissible="!busy" @update:open="!$event && emit('close')">
    <template #body>
      <div class="credit-form">
        <USkeleton v-if="loading" class="h-12" />
        <template v-else-if="!loadError">
          <UFormField v-if="payment?.provider === 'builtin_usdt'" :label="t('adminUsdt.evidence')"><USelect v-model="selected" :items="items" :disabled="busy" class="w-full" /></UFormField>
          <p v-if="transfer" class="credit-evidence"><code>{{ transfer.recipient }}</code><code>{{ transfer.txHash }}</code></p>
          <UFormField :label="t('adminUsdt.auditReason')" required><UTextarea v-model.trim="reason" :maxlength="500" :disabled="busy" class="w-full" /></UFormField>
        </template>
        <InlineNotice v-if="loadError" tone="warning">{{ loadError }} <UButton :label="t('adminSection.retry')" variant="link" @click="load" /></InlineNotice>
        <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      </div>
    </template>
    <template #footer>
      <UButton :label="t('common.cancel')" color="neutral" variant="outline" :disabled="busy" @click="emit('close')" />
      <UButton :label="t(transfer ? 'adminUsdt.credit' : 'adminUserProfile.issueCourtesyCredit')" :loading="busy" :disabled="busy || loading || !!loadError || reason.length < 3 || (selected !== 'courtesy' && !transfer)" @click="emit('confirm', transfer?.id ?? '', reason)" />
    </template>
  </UModal>
</template>

<style scoped>
.credit-form { display: grid; gap: .8rem; }
.credit-evidence { display: grid; gap: .3rem; overflow-wrap: anywhere; font-size: .75rem; }
</style>
