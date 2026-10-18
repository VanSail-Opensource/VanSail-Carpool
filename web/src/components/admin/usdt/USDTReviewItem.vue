<script setup lang="ts">
import { shallowRef } from 'vue'
import type { USDTTransfer } from '@/api/usdt'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import { paymentChannelLabel } from '@/components/billing/paymentOptions'
import { networkRails } from './configForm'
const props = defineProps<{ transfer: USDTTransfer; busy: boolean }>()
const emit = defineEmits<{ dismiss: [id: string, reason: string] }>()
const { t } = useI18n()
const reason = shallowRef('')
function dismiss(): void { if (reason.value.trim().length >= 3) emit('dismiss', props.transfer.id, reason.value.trim()) }
</script>

<template>
  <article class="usdt-review-item">
    <div class="usdt-toolbar">
      <strong>{{ transfer.amount }} {{ transfer.currency }}</strong>
      <span>{{ paymentChannelLabel(networkRails[transfer.network].replace('usdt.', `${transfer.currency.toLowerCase()}.`), transfer.network) }}</span>
      <small>{{ formatDateTime(transfer.occurredAt) }}</small>
    </div>
    <dl class="usdt-evidence">
      <div><dt>{{ t('adminUsdt.recipient') }}</dt><dd><code>{{ transfer.recipient }}</code></dd></div>
      <div><dt>{{ t('adminUsdt.transaction') }}</dt><dd><code>{{ transfer.txHash }}</code></dd></div>
      <div><dt>{{ t('adminUsdt.reason') }}</dt><dd>{{ t(`adminUsdt.reviewReasons.${transfer.reason}`) }}</dd></div>
    </dl>
    <div class="usdt-form-grid">
      <UFormField :label="t('adminUsdt.dismissReason')"><UInput v-model="reason" :maxlength="500" :disabled="busy" /></UFormField>
      <UButton icon="i-ph-check" :label="t('adminUsdt.dismiss')" :loading="busy" :disabled="reason.trim().length < 3 || busy" @click="dismiss" />
    </div>
  </article>
</template>
