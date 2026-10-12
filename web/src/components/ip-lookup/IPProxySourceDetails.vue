<script setup lang="ts">
import type { IPLookupReport } from '@/api/ipLookup'
defineProps<{ source: NonNullable<IPLookupReport['proxy']>['sources'][number] }>()
</script>

<template>
  <div class="proxy-source-details">
    <div class="source-heading"><strong>{{ source.name }}</strong><span>{{ $t(`ipLookup.probeStatuses.${source.status}`) }}</span><span>{{ $t('ipLookup.inventoryIPs', { count: source.count }) }}</span></div>
    <a v-if="source.url" :href="source.url" target="_blank" rel="noopener noreferrer">{{ source.url }}</a>
    <p v-if="source.hosts !== undefined">{{ $t('ipLookup.inventoryProgress', { done: source.processed ?? 0, total: source.hosts }) }}<span>{{ $t('ipLookup.inventorySkipped', { count: source.skipped ?? 0 }) }}</span><span>{{ $t('ipLookup.inventoryDNSFailed', { count: source.dnsFailed ?? 0 }) }}</span></p>
    <p v-if="source.errorCode" class="source-error">{{ $t(`ipLookup.sourceErrors.${source.errorCode}`) }}</p>
    <time v-if="source.observedAt" :datetime="source.observedAt">{{ new Date(source.observedAt).toLocaleString() }}</time>
  </div>
</template>

<style scoped>
.proxy-source-details { min-width: 0; padding-block: 0.45rem; font-size: 0.75rem; border-bottom: 1px solid var(--line); }
.source-heading, .proxy-source-details p { display: flex; flex-wrap: wrap; gap: 0.3rem 0.7rem; margin: 0.3rem 0; }
.source-heading strong { font-weight: 500; }
.proxy-source-details a { display: block; color: var(--ui-info); overflow-wrap: anywhere; font-size: 0.7rem; }
.proxy-source-details time, .proxy-source-details p, .source-heading span { color: var(--text-muted); }
.proxy-source-details .source-error { color: var(--warning); }
</style>
