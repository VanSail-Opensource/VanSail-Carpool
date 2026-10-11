<script setup lang="ts">
import { computed } from 'vue'
import type { IPLookupReport } from '@/api/ipLookup'
const props = defineProps<{ result: NonNullable<IPLookupReport['proxy']> }>()
const names = computed(() => props.result.matches.map(id => props.result.sources.find(source => source.id === id)?.name ?? id))
</script>

<template>
  <section class="proxy-evidence"><h3>{{ $t('ipLookup.proxyHeading') }}</h3><p>{{ names.length ? names.join(', ') : $t(result.status === 'success' ? 'ipLookup.noProxyMatch' : 'ipLookup.proxyIncomplete') }}</p><details><summary>{{ $t('ipLookup.proxyCoverage') }}</summary><dl><div v-for="source in result.sources" :key="source.id"><dt>{{ source.name }}</dt><dd>{{ $t(`ipLookup.probeStatuses.${source.status}`) }} <small>{{ source.count }}</small></dd></div></dl></details></section>
</template>

<style scoped>
.proxy-evidence { border-top: 1px solid var(--line); padding-top: 0.6rem; font-size: 0.78rem; }
.proxy-evidence h3 { margin: 0 0 0.3rem; font-size: 0.85rem; }
.proxy-evidence p { margin: 0.3rem 0; }
.proxy-evidence summary, .proxy-evidence dt, .proxy-evidence small { color: var(--text-muted); }
.proxy-evidence summary { cursor: pointer; }
.proxy-evidence dl { margin: 0.3rem 0; }
.proxy-evidence dl > div { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 0.5rem; padding: 0.2rem 0; border-bottom: 1px solid var(--line); }
.proxy-evidence dd { margin: 0; }
</style>
