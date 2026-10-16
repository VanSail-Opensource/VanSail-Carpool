<script setup lang="ts">
import { computed } from 'vue'
import { xnodeMemberApi } from '@/api/xnodeMember'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'

const { t } = useI18n()
const { value, loading, error, refresh } = useSubscriptionData(xnodeMemberApi.metrics, 5000, 'xnode.loadFailed')
const reasons = computed(() => [...new Set(value.value?.reasons.map(item => {
  const key = `memberNodes.reasons.${item.reason}`
  return t(key) === key ? t('memberNodes.reasons.unknown') : t(key)
}) ?? [])])
const rate = (bytes: number) => t('statistics.perSecond', { value: formatBytes(Number.isFinite(bytes) ? bytes : 0) })
</script>

<template>
  <section class="section-block" aria-live="polite">
    <div class="section-heading"><h2>{{ t('memberNodes.live') }}</h2><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :loading="loading" :aria-label="t('common.refresh')" @click="refresh()" /></div>
    <p v-if="loading && !value" class="text-muted">{{ t('common.loading') }}</p>
    <template v-if="value">
      <dl class="grid grid-cols-3 gap-3 text-sm"><div><dt class="text-muted">{{ t('xnode.connections') }}</dt><dd>{{ value.connections }}</dd></div><div><dt class="text-muted">{{ t('xnode.upload') }}</dt><dd>{{ rate(value.uploadBytesPerSecond) }}</dd></div><div><dt class="text-muted">{{ t('xnode.download') }}</dt><dd>{{ rate(value.downloadBytesPerSecond) }}</dd></div></dl>
      <p v-for="reason in reasons" :key="reason" class="text-sm text-warning mt-2">{{ reason }}</p>
      <p v-if="value.incomplete" class="text-xs text-muted mt-2">{{ t('memberNodes.incomplete') }}</p>
    </template>
    <p v-if="error" role="status" class="text-sm text-muted">{{ error }}</p>
  </section>
</template>
