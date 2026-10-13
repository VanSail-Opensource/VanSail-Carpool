<script setup lang="ts">
import { shallowRef } from 'vue'
import type { USDTTransfer } from '@/api/usdt'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import { paymentChannelLabel } from '@/components/billing/paymentOptions'
import { networkRails } from './configForm'
const props = defineProps<{ transfer: USDTTransfer; busy: boolean }>()
const emit = defineEmits<{ resolve: [id: string, orderId: string, reason: string] }>()
const { t } = useI18n()
const orderId = shallowRef(props.transfer.orderId)
const reason = shallowRef('')
function resolve(): void { if (orderId.value.trim() && reason.value.trim()) emit('resolve', props.transfer.id, orderId.value.trim(), reason.value.trim()) }
</script>

<template>
  <article class="usdt-review-item">
    <div class="usdt-toolbar">
      <strong>{{ transfer.amount }} {{ t('adminUsdt.currency') }}</strong>
      <span>{{ paymentChannelLabel(networkRails[transfer.network], transfer.network) }}</span>
      <small>{{ formatDateTime(transfer.occurredAt) }}</small>
    </div>
    <dl class="usdt-evidence">
      <div><dt>{{ t('adminUsdt.recipient') }}</dt><dd><code>{{ transfer.recipient }}</code></dd></div>
      <div><dt>{{ t('adminUsdt.transaction') }}</dt><dd><code>{{ transfer.txHash }}</code></dd></div>
      <div><dt>{{ t('adminUsdt.reason') }}</dt><dd>{{ t(`adminUsdt.reviewReasons.${transfer.reason}`) }}</dd></div>
    </dl>
    <div v-if="transfer.reason !== 'USDT_ALREADY_CREDITED'" class="usdt-form-grid">
      <UFormField :label="t('adminUsdt.orderId')"><UInput v-model="orderId" :disabled="busy" /></UFormField>
      <UFormField :label="t('adminUsdt.auditReason')"><UInput v-model="reason" :maxlength="500" :disabled="busy" /></UFormField>
      <UButton icon="i-ph-check" :label="t('adminUsdt.credit')" :loading="busy" :disabled="!orderId.trim() || !reason.trim() || busy" @click="resolve" />
    </div>
  </article>
</template>
