<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { AdminEntitlement } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatBytes, formatMoney, moneyFromTxbInput } from '@/utils/format'
import { useAdminRefundQuote } from './useAdminRefundQuote'

const open = defineModel<boolean>('open', { required: true })
const props = withDefaults(defineProps<{ item: AdminEntitlement | null; busy?: boolean; error?: string | null }>(), { busy: false, error: null })
const emit = defineEmits<{ refund: [body: { reason: string; amountTxbMinor: string }] }>()
const { t } = useI18n()
const reason = shallowRef('')
const refundQuote = useAdminRefundQuote(open, () => props.item)
const amount = refundQuote.amount
const maximum = computed(() => props.item?.price.minor ?? '0')
const amountMinor = computed(() => moneyFromTxbInput(amount.value))
const valid = computed(() => reason.value.length >= 4 && amountMinor.value !== '' && BigInt(amountMinor.value || '0') > 0n && BigInt(amountMinor.value || '0') <= BigInt(maximum.value))
const workflowState = computed(() => props.busy ? 'processing' : props.error ? `error:${props.error}` : 'review')
const { reducedMotion } = useMotionPreferences()

watch(() => [open.value, props.item?.id, props.item?.userId], () => {
  if (!open.value || !props.item) return
  reason.value = ''
}, { immediate: true })
</script>

<template>
  <UModal v-model:open="open" :title="t('adminUserProfile.refundTitle')" :description="t('adminUserProfile.refundHint')" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }">
    <template #body>
      <AnimatePresence mode="wait" :initial="false">
        <motion.div :key="workflowState" class="form-stack" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
          <USkeleton v-if="refundQuote.loading.value" class="h-12" />
          <dl v-if="refundQuote.quote.value?.suggestedRefund" class="refund-quote-facts">
            <div><dt>{{ t('adminRefundQuote.paid') }}</dt><dd>{{ formatMoney(refundQuote.quote.value.paid) }}</dd></div>
            <div><dt>{{ t('adminRefundQuote.traffic') }}</dt><dd>{{ formatBytes(refundQuote.quote.value.usedTrafficBytes ?? '0') }} / {{ formatBytes(refundQuote.quote.value.allocatedTrafficBytes ?? '0') }}</dd></div>
          </dl>
          <p v-if="refundQuote.quote.value?.suggestedRefund" class="text-sm text-muted">{{ t('adminRefundQuote.hint') }}</p>
          <InlineNotice v-if="refundQuote.error.value" tone="warning">{{ refundQuote.error.value }}<UButton variant="link" :label="t('common.tryAgain')" :disabled="refundQuote.loading.value" @click="refundQuote.retry()" /></InlineNotice>
          <TxbAmountField id="admin-entitlement-refund" v-model="amount" :label="t('adminUserProfile.refundAmount')" min-minor="1" :max-minor="maximum" required />
          <UFormField name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="reason" :rows="3" :minlength="4" :maxlength="300" :placeholder="t('adminReason.placeholder')" /></UFormField>
          <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        </motion.div>
      </AnimatePresence>
    </template>
    <template #footer="{ close }">
      <UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="close" />
      <UButton color="warning" :label="t('adminUserProfile.issueRefund')" :disabled="!valid || busy" :loading="busy" @click="amountMinor && emit('refund', { reason, amountTxbMinor: amountMinor })" />
    </template>
  </UModal>
</template>

<style scoped>
.refund-quote-facts { display: grid; gap: 0.5rem; margin: 0; font-size: 0.85rem; }
.refund-quote-facts div { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 0.5rem; }
.refund-quote-facts dt { color: var(--text-muted); }
.refund-quote-facts dd { margin: 0; }
</style>
