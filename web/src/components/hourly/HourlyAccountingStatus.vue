<script setup lang="ts">
import { computed } from 'vue'
import type { HourlyView } from '@/api/hourly'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'

const props = withDefaults(defineProps<{ usage: HourlyView['usage']; showHistorical?: boolean }>(), { showHistorical: false })
const { t } = useI18n()
const messages = computed(() => {
  const usage = props.usage
  if (!usage) return []
  const accounting = usage.accounting
  if (!accounting) return usage.accountingPending ? [{ kind: 'legacy', text: t('hourly.accountingPending') }] : []
  const result: { kind: string; text: string }[] = []
  if (BigInt(accounting.openReservedBytes) > 0n) {
    result.push({ kind: 'open', text: t('hourly.accountingReserved', { bytes: formatBytes(accounting.openReservedBytes) }) })
  }
  if (BigInt(accounting.recoveryUnknownBytes) > 0n) {
    result.push({ kind: 'recovery', text: t('hourly.accountingRecoveryUnknown', { bytes: formatBytes(accounting.recoveryUnknownBytes) }) })
  }
  if (props.showHistorical && BigInt(accounting.historicalRecoveryUnknownBytes) > 0n) {
    result.push({ kind: 'historical-recovery', text: t('hourly.accountingHistoricalUnknown', { bytes: formatBytes(accounting.historicalRecoveryUnknownBytes) }) })
  }
  if (props.showHistorical && BigInt(accounting.historicalUnsealedCount) > 0n) {
    result.push({ kind: 'historical-unsealed', text: t('hourly.accountingHistoricalUnsealed', { count: accounting.historicalUnsealedCount, bytes: formatBytes(accounting.historicalUnsealedReservedBytes) }) })
  }
  return result
})
</script>

<template>
  <div v-if="messages.length" class="hourly-accounting" role="status" data-hourly-accounting>
    <p v-for="message in messages" :key="message.kind" class="hourly-accounting__message" :data-hourly-accounting-kind="message.kind">{{ message.text }}</p>
  </div>
</template>

<style scoped>
.hourly-accounting { color: var(--text-muted); font-size: .78rem; line-height: 1.55; margin-block: .7rem; overflow-wrap: anywhere; }
.hourly-accounting__message { margin: .45rem 0 0; }
</style>
