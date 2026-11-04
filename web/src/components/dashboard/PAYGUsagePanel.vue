<script setup lang="ts">
import { computed } from 'vue'
import { paygApi } from '@/api/payg'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { t } from '@/i18n'
import PAYGUsagePie from './PAYGUsagePie.vue'
const { value, loading, refreshing, error, refresh } = useSubscriptionData(paygApi.history, 60_000, 'paygHistory.loadFailed')
const stale = computed(() => !!value.value && (error.value || Date.now() - new Date(value.value.to).getTime() > 120_000))
</script>

<template>
  <section class="section-block grid gap-3 min-w-0">
    <header class="flex justify-between items-center gap-2"><h2>{{ t('paygHistory.title') }}</h2><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="refreshing" @click="refresh()" /></header>
    <p class="text-xs text-muted">{{ t('paygHistory.hint') }}</p>
    <UProgress v-if="loading" :aria-label="t('common.loading')" />
    <UAlert v-if="error" color="warning" :title="error" />
    <UAlert v-if="stale" color="warning" :title="t('paygHistory.stale')" />
    <template v-if="value">
      <UAlert v-if="value.truncated" color="warning" :title="t('paygHistory.truncated')" />
      <PAYGUsagePie :history="value" />
    </template>
  </section>
</template>
