<script setup lang="ts">
import { computed, onMounted, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeMemberApi } from '@/api/xnodeMember'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { useI18n } from '@/i18n'
import { activityDeadline, memberActivity, type ActivityStatus } from './memberActivity'
import MemberLiveValues from './MemberLiveValues.vue'

const emit = defineEmits<{ activity: [value: ActivityStatus] }>()

const { t } = useI18n()
const { value: observation, loading, refreshing, error, refresh } = useSubscriptionData(async (signal) => {
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
const statusLabel = computed(() => loading.value && !value.value ? t('common.loading') : t(`dashboard.${activity.value === 'unavailable' ? 'activityUnavailable' : activity.value}`))
</script>

<template>
  <section class="section-block member-live-metrics" :aria-label="t('memberNodes.live')">
    <div class="section-heading member-live-metrics__heading">
      <div><h2>{{ t('memberNodes.live') }}</h2><p class="member-live-metrics__status" :data-activity="activity" role="status">{{ statusLabel }}</p></div>
      <UButton class="member-live-metrics__refresh" variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :disabled="loading || refreshing" :aria-label="t('common.refresh')" @click="refresh()" />
    </div>
    <MemberLiveValues :metrics="activity !== 'unavailable' ? value : null" />
    <div class="member-live-metrics__notices" aria-live="polite">
      <p v-if="error" class="text-sm text-muted">{{ error }}</p>
      <template v-if="value && activity !== 'unavailable'">
        <p v-for="reason in reasons" :key="reason" class="text-sm text-warning">{{ reason }}</p>
        <p v-if="value.incomplete" class="text-xs text-muted">{{ t('memberNodes.incomplete') }}</p>
      </template>
    </div>
  </section>
</template>

<style scoped>
.member-live-metrics { min-width: 0; }
.member-live-metrics__heading { align-items: start; gap: .75rem; }
.member-live-metrics__heading > div { min-width: 0; }
.member-live-metrics__status { margin: .35rem 0 0; color: var(--text-muted); font-size: .72rem; overflow-wrap: anywhere; }
.member-live-metrics__status[data-activity='online'] { color: var(--accent); }
.member-live-metrics__refresh { flex: 0 0 auto; min-width: 44px; min-height: 44px; }
.member-live-metrics__notices p { margin: .75rem 0 0; }
</style>
