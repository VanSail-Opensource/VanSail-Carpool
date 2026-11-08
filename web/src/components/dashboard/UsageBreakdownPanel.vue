<script setup lang="ts">
import { computed } from 'vue'
import { usageBreakdownApi } from '@/api/usageBreakdown'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { getLocale, t } from '@/i18n'
import UsageBreakdownPie from './UsageBreakdownPie.vue'

const { value, loading, refreshing, error, refresh } = useSubscriptionData(usageBreakdownApi.get, 60_000, 'usageBreakdown.loadFailed')
const stale = computed(() => !!value.value && Date.now() - new Date(value.value.to).getTime() > 120_000)
const coverageSince = computed(() => value.value?.coverage.since
  ? new Intl.DateTimeFormat(getLocale(), { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' }).format(new Date(value.value.coverage.since)) : '')
</script>

<template>
  <section class="section-block grid gap-3 min-w-0" :aria-busy="loading || refreshing" data-usage-panel>
    <header class="flex justify-between items-center gap-2"><h2>{{ t('usageBreakdown.title') }}</h2><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="refreshing" data-haptic="refresh" @click="refresh()" /></header>
    <p class="text-xs text-muted">{{ t('usageBreakdown.hint') }}</p>
    <div v-if="loading" role="status" aria-live="polite"><span class="sr-only">{{ t('common.loading') }}</span><UProgress aria-hidden="true" /></div>
    <UAlert v-if="error" color="warning" :title="value ? t('usageBreakdown.stale') : error" :description="value ? error : undefined" />
    <UAlert v-else-if="stale" color="warning" :title="t('usageBreakdown.stale')" />
    <template v-if="value">
      <p v-if="!value.coverage.complete" class="text-xs text-muted" data-usage-coverage>{{ coverageSince ? t('usageBreakdown.incompleteSince', { since: coverageSince }) : t('usageBreakdown.incomplete') }}</p>
      <p v-if="value.coverage.gapCount !== '0'" class="text-xs text-muted">{{ t('usageBreakdown.gaps', { count: value.coverage.gapCount }) }}</p>
      <p v-if="value.coverage.truncated" class="text-xs text-muted">{{ t('usageBreakdown.truncated') }}</p>
      <UsageBreakdownPie :history="value" />
    </template>
  </section>
</template>
