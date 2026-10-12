<script setup lang="ts">
import IPProxySourceDetails from '@/components/ip-lookup/IPProxySourceDetails.vue'
import { ipLookupErrorText } from '@/components/ip-lookup/presentation'
import { useProxySourceTest } from './useProxySourceTest'
const props = defineProps<{ sourceId: string; dirty: boolean; disabled?: boolean }>()
const { test, busy, errorCode, start } = useProxySourceTest(() => props.sourceId)
</script>

<template>
  <div class="source-test-control">
    <UButton type="button" color="neutral" variant="outline" icon="i-ph-play" :label="$t('ipLookup.testSource')" :loading="busy" :disabled="disabled || dirty || busy" @click="start" />
    <p v-if="dirty">{{ $t('ipLookup.sourceSaveFirst') }}</p>
    <p v-if="errorCode" role="alert">{{ ipLookupErrorText(errorCode) }}</p>
    <div v-if="test" aria-live="polite"><p v-if="busy">{{ $t(`ipLookup.sourcePhases.${test.result.phase ?? 'download'}`) }}</p><IPProxySourceDetails :source="test.result" /></div>
  </div>
</template>

<style scoped>
.source-test-control { display: contents; font-size: 0.75rem; }
.source-test-control > p, .source-test-control > div { grid-column: 1 / -1; min-width: 0; }
.source-test-control > button { grid-column: 1; }
@media (min-width: 900px) { .source-test-control > button { grid-column: 4; grid-row: 1; min-width: 5rem; } }
.source-test-control p { margin: 0.35rem 0; color: var(--text-muted); }
</style>
