<script setup lang="ts">
import { computed } from 'vue'
import { usageBreakdownApi } from '@/api/usageBreakdown'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { t } from '@/i18n'
import UsageBreakdownPie from './UsageBreakdownPie.vue'

const { value, loading, refreshing, error, refresh } = useSubscriptionData(usageBreakdownApi.get, 60_000, 'usageBreakdown.loadFailed')
const stale = computed(() => !!value.value && Date.now() - new Date(value.value.to).getTime() > 120_000)
</script>

<template>
  <section class="section-block grid gap-3 min-w-0" :aria-busy="loading || refreshing" data-usage-panel>
    <header class="flex justify-between items-center gap-2"><h2>{{ t('usageBreakdown.title') }}</h2><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="refreshing" data-haptic="refresh" @click="refresh()" /></header>
    <div v-if="loading" role="status" aria-live="polite"><span class="sr-only">{{ t('common.loading') }}</span><UProgress aria-hidden="true" /></div>
    <UAlert v-if="error" color="warning" :title="value ? t('usageBreakdown.stale') : error" :description="value ? error : undefined" />
    <UAlert v-else-if="stale" color="warning" :title="t('usageBreakdown.stale')" />
    <UsageBreakdownPie v-if="value" :history="value" />
  </section>
</template>
