<script setup lang="ts">
import { computed, onMounted, shallowRef, watch } from 'vue'

import type { MemberRefundQuote, OperationStatus, Purchase } from '@/api/types'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { type PurchaseOperationKind, usePurchaseOperations } from '@/composables/usePurchaseOperations'
import { useTelegramBackButton } from '@/composables/useTelegramBackButton'
import { t } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import { formatMemberMoney } from '@/utils/displayCurrency'
import AutoRenewalControl from './AutoRenewalControl.vue'

const props = defineProps<{ purchase: Purchase }>()
const emit = defineEmits<{ changed: [] }>()
const dialogKind = shallowRef<PurchaseOperationKind | null>(null)
const operation = usePurchaseOperations(() => props.purchase.id)
let emittedReceiptId = ''

const dialogOpen = computed({
  get: () => dialogKind.value !== null,
  set: (open) => { if (!open && !operation.mutating.value) dialogKind.value = null },
})
const ownsBack = computed(() => dialogOpen.value)
const currentQuote = computed<MemberRefundQuote | null>(() => operation.refundQuote.value)
const quoteEligible = computed(() => currentQuote.value?.eligible === true)
const refundEligible = computed(() => operation.refundQuote.value?.eligible === true)
const operationTone = computed(() => operation.receipt.value?.status === 'succeeded' ? 'success' : 'warning')
const operationMessage = computed(() => t(`purchaseOperations.operation.refund.${operation.receipt.value?.status ?? 'queued'}`))

function quoteAmount(quote: MemberRefundQuote): string {
  return formatMemberMoney(quote.refund)
}

function reasonMessage(reason: string | null | undefined): string {
  if (!reason) return t('purchaseOperations.ineligible')
  const key = `purchaseOperations.reason.${reason}`
  const message = t(key)
  return message === key ? t('purchaseOperations.ineligible') : message
}

function statusLabel(status: OperationStatus): string {
  return t(`operations.status.${status}`)
}

async function openQuote(kind: PurchaseOperationKind): Promise<void> {
  if (operation.blocksMutations.value) {
    dialogKind.value = operation.activeKind.value ?? kind
    return
  }
  operation.dismissOperation()
  dialogKind.value = kind
  await operation.loadQuote()
}

function closeDialog(): void {
  if (!operation.mutating.value) dialogKind.value = null
}

watch(() => operation.receipt.value, (receipt) => {
  if (!receipt || receipt.id === emittedReceiptId || !['succeeded', 'compensated'].includes(receipt.status)) return
  emittedReceiptId = receipt.id
  emit('changed')
  void operation.loadRefundEligibility()
})
watch(() => props.purchase.id, () => {
  dialogKind.value = null
  emittedReceiptId = ''
  operation.reset()
  void operation.loadRefundEligibility()
})

onMounted(() => void operation.loadRefundEligibility())
useTelegramBackButton(ownsBack, closeDialog)
</script>

<template>
  <div class="purchase-actions">
    <div class="purchase-actions__row">
      <AutoRenewalControl class="purchase-actions__primary" :purchase="purchase" @changed="emit('changed')" />
      <USkeleton v-if="operation.refundEligibilityLoading.value" class="purchase-actions__refund h-11" />
      <UButton
        v-else-if="refundEligible"
        class="purchase-actions__refund"
        color="warning"
        variant="soft"
        icon="i-ph-arrow-u-up-left"
        :disabled="operation.blocksMutations.value"
        :label="$t('purchaseOperations.refundAction')"
        data-haptic="open"
        @click="openQuote('refund')"
      />
    </div>

    <InlineNotice v-if="operation.receipt.value && !dialogOpen" :tone="operationTone" :title="statusLabel(operation.receipt.value.status)">
      {{ operationMessage }}
    </InlineNotice>
    <UButton
      v-if="operation.receipt.value && operation.error.value && !dialogOpen"
      color="neutral"
      variant="outline"
      icon="i-ph-arrow-clockwise"
      :loading="operation.checking.value"
      :label="$t('operations.checkStatus')"
      data-haptic="retry"
      @click="operation.refresh"
    />

    <UModal
      v-model:open="dialogOpen"
      :title="$t('purchaseOperations.refund.title')"
      :description="$t('purchaseOperations.refund.description')"
      :dismissible="!operation.mutating.value"
      :close="false"
      :ui="{ header: 'tg-overlay-header--centered', wrapper: 'tg-overlay-copy--centered', footer: 'justify-end' }"
    >
      <template #body>
        <div class="purchase-operation-dialog">
          <USkeleton v-if="operation.quoteLoading.value" class="h-28" />
          <template v-else-if="currentQuote">
            <dl class="purchase-operation-quote">
              <div><dt>{{ $t('purchaseOperations.amount') }}</dt><dd>{{ quoteAmount(currentQuote) }}</dd></div>
              <div v-if="'eligibilityExpiresAt' in currentQuote && currentQuote.eligibilityExpiresAt"><dt>{{ $t('purchaseOperations.eligibleUntil') }}</dt><dd>{{ formatDateTime(currentQuote.eligibilityExpiresAt) }}</dd></div>
            </dl>
            <InlineNotice v-if="!currentQuote.eligible" tone="warning">{{ reasonMessage(currentQuote.reasonCode) }}</InlineNotice>
          </template>
          <InlineNotice v-if="operation.receipt.value" :tone="operationTone" :title="statusLabel(operation.receipt.value.status)">
            {{ operationMessage }}
          </InlineNotice>
          <InlineNotice v-if="operation.error.value" tone="warning">{{ operation.error.value }}</InlineNotice>
          <UButton
            v-if="operation.receipt.value && operation.error.value"
            color="neutral"
            variant="outline"
            icon="i-ph-arrow-clockwise"
            :loading="operation.checking.value"
            :label="$t('operations.checkStatus')"
            data-haptic="retry"
            @click="operation.refresh"
          />
        </div>
      </template>
      <template #footer>
        <UButton color="neutral" variant="outline" :disabled="operation.mutating.value" :label="$t('common.close')" data-haptic="dismiss" @click="closeDialog" />
        <UButton
          v-if="!operation.receipt.value"
          color="warning"
          icon="i-ph-arrow-u-up-left"
          :loading="operation.mutating.value"
          :disabled="operation.quoteLoading.value || !quoteEligible"
          :label="$t('purchaseOperations.refund.confirm')"
          data-haptic="destructive"
          @click="dialogKind && operation.start(dialogKind)"
        />
      </template>
    </UModal>
  </div>
</template>

<style scoped>
.purchase-actions, .purchase-operation-dialog { display: grid; gap: 0.75rem; }
.purchase-actions__row { display: flex; flex-wrap: wrap; align-items: stretch; gap: 0.55rem; }
.purchase-actions__primary { min-width: 0; flex: 1 1 auto; }
.purchase-actions__refund { min-width: 0; flex: 1 1 8rem; }
.purchase-operation-quote { display: grid; gap: 0.5rem; margin: 0; }
.purchase-operation-quote > div { display: flex; align-items: baseline; justify-content: space-between; gap: 1rem; padding-bottom: 0.5rem; border-bottom: 1px solid var(--line); }
.purchase-operation-quote dt { color: var(--text-faint); font-size: 0.72rem; }
.purchase-operation-quote dd { margin: 0; font-family: var(--font-mono); font-size: 0.78rem; font-weight: 700; text-align: right; overflow-wrap: anywhere; }
</style>
