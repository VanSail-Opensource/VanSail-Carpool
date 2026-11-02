<script setup lang="ts">
import { computed, onMounted, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeMemberApi } from '@/api/xnodeMember'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { activityDeadline, memberActivity, type ActivityStatus } from './memberActivity'

const emit = defineEmits<{ activity: [value: ActivityStatus] }>()

const { t } = useI18n()
const { value: observation, loading, error, refresh } = useSubscriptionData(async (signal) => {
  const startedAt = globalThis.performance.now()
  const metrics = await xnodeMemberApi.metrics(signal)
  return { metrics, deadline: activityDeadline(metrics, startedAt) }
}, 5000, 'xnode.loadFailed')
const value = computed(() => observation.value?.metrics ?? null)
const now = shallowRef(globalThis.performance.now())
const invalidated = shallowRef(false)
let expiryTimer: ReturnType<typeof globalThis.setTimeout> | undefined
const activity = computed(() => invalidated.value ? 'unavailable' : memberActivity(value.value, error.value, now.value, observation.value?.deadline ?? NaN))
watch(observation, next => { if (next) invalidated.value = false })
watch([observation, error], ([next]) => {
  if (expiryTimer) globalThis.clearTimeout(expiryTimer)
  now.value = globalThis.performance.now()
  const delay = (next?.deadline ?? NaN) - now.value
  if (!error.value && Number.isFinite(delay) && delay > 0) expiryTimer = globalThis.setTimeout(() => { now.value = globalThis.performance.now() }, Math.ceil(delay))
}, { immediate: true })
watch(activity, status => emit('activity', status), { immediate: true })
function invalidateActivity(): void { invalidated.value = true }
onMounted(() => {
  globalThis.document.addEventListener('visibilitychange', invalidateActivity)
  globalThis.window.addEventListener('focus', invalidateActivity)
})
onScopeDispose(() => {
  globalThis.document.removeEventListener('visibilitychange', invalidateActivity)
  globalThis.window.removeEventListener('focus', invalidateActivity)
  if (expiryTimer) globalThis.clearTimeout(expiryTimer)
  emit('activity', 'unavailable')
})
const reasons = computed(() => [...new Set(value.value?.reasons.map(item => {
  const key = `memberNodes.reasons.${item.reason}`
  return t(key) === key ? t('memberNodes.reasons.unknown') : t(key)
}) ?? [])])
const rate = (bytes: number) => t('statistics.perSecond', { value: formatBytes(Number.isFinite(bytes) ? bytes : 0) })
const connectionCounts = computed(() => ({ active: value.value?.connections ?? 0, total: value.value?.totalConnections ?? t('common.notAvailable') }))
</script>

<template>
  <section class="section-block" aria-live="polite">
    <div class="section-heading"><h2>{{ t('memberNodes.live') }}</h2><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :loading="loading" :aria-label="t('common.refresh')" @click="refresh()" /></div>
    <p v-if="loading && !value" class="text-muted">{{ t('common.loading') }}</p>
    <template v-if="value && activity !== 'unavailable'">
      <dl class="grid grid-cols-3 gap-3 text-sm">
        <div :title="t('memberNodes.connectionsHint')"><dt class="text-muted">{{ t('xnode.connections') }}</dt><dd class="tabular-nums" :aria-label="t('memberNodes.connectionsLabel', connectionCounts)">{{ t('memberNodes.connectionCounts', connectionCounts) }}</dd></div>
        <div><dt class="text-muted">{{ t('xnode.upload') }}</dt><dd>{{ rate(value.uploadBytesPerSecond) }}</dd></div>
        <div><dt class="text-muted">{{ t('xnode.download') }}</dt><dd>{{ rate(value.downloadBytesPerSecond) }}</dd></div>
      </dl>
      <p v-for="reason in reasons" :key="reason" class="text-sm text-warning mt-2">{{ reason }}</p>
      <p v-if="value.incomplete" class="text-xs text-muted mt-2">{{ t('memberNodes.incomplete') }}</p>
    </template>
    <p v-else-if="value && !error" class="text-xs text-muted">{{ t('memberNodes.incomplete') }}</p>
    <p v-if="error" role="status" class="text-sm text-muted">{{ error }}</p>
  </section>
</template>
