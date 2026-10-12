<script setup lang="ts">
import type { IPLookupProxySourceCacheStatus } from '@/api/ipLookup'
defineProps<{ source?: IPLookupProxySourceCacheStatus; dirty: boolean; disabled?: boolean; starting: boolean }>()
defineEmits<{ refresh: [] }>()
</script>

<template>
  <div class="source-test-control">
    <UButton type="button" color="neutral" variant="outline" icon="i-ph-arrow-clockwise" :label="$t(source?.errorCode ? 'ipLookup.retrySource' : 'ipLookup.refreshSource')" :loading="starting || source?.refreshing" :disabled="disabled || dirty || starting || source?.refreshing" @click="$emit('refresh')" />
    <div class="source-cache-status" aria-live="polite">
      <span>{{ $t(`ipLookup.probeStatuses.${source?.status ?? 'pending'}`) }}</span>
      <span>{{ $t('ipLookup.inventoryIPs', { count: source?.count ?? 0 }) }}</span>
      <span v-if="source?.stale && source.enabled && !source.refreshing">{{ $t('ipLookup.cacheStale') }}</span>
      <span v-if="source?.refreshing">{{ $t(`ipLookup.sourcePhases.${source.phase ?? 'download'}`) }}</span>
      <time v-if="source?.observedAt" :datetime="source.observedAt">{{ $t('ipLookup.sourceUpdated', { date: new Date(source.observedAt).toLocaleString() }) }}</time>
      <time v-if="source?.nextRefreshAt && source.enabled" :datetime="source.nextRefreshAt">{{ $t('ipLookup.sourceNextRefresh', { date: new Date(source.nextRefreshAt).toLocaleString() }) }}</time>
      <span v-if="source?.errorCode" class="source-error">{{ $t(`ipLookup.sourceErrors.${source.errorCode}`) }}</span>
      <span v-if="dirty">{{ $t('ipLookup.sourceSaveFirst') }}</span>
    </div>
  </div>
</template>

<style scoped>
.source-test-control { display: contents; font-size: 0.75rem; }
.source-cache-status { display: flex; flex-wrap: wrap; gap: 0.25rem 0.7rem; grid-column: 1 / -1; min-width: 0; color: var(--text-muted); }
.source-cache-status .source-error { color: var(--warning); }
.source-test-control > button { grid-column: 1; }
@media (min-width: 900px) { .source-test-control > button { grid-column: 4; grid-row: 1; min-width: 5rem; } }
</style>
