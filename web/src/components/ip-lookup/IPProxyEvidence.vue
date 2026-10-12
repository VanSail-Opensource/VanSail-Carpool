<script setup lang="ts">
import { computed } from 'vue'
import IPProxySourceDetails from './IPProxySourceDetails.vue'
import type { IPLookupReport } from '@/api/ipLookup'
const props = defineProps<{ result: NonNullable<IPLookupReport['proxy']> }>()
const names = computed(() => props.result.matches.map(id => props.result.sources.find(source => source.id === id)?.name ?? id))
</script>

<template>
  <section class="proxy-evidence"><h3>{{ $t('ipLookup.proxyHeading') }}</h3><p>{{ names.length ? names.join(', ') : $t(result.status === 'success' ? 'ipLookup.noProxyMatch' : 'ipLookup.proxyIncomplete') }}</p><IPProxySourceDetails v-for="source in result.sources" :key="source.id" :source="source" /></section>
</template>

<style scoped>
.proxy-evidence { border-top: 1px solid var(--line); padding-top: 0.6rem; font-size: 0.78rem; }
.proxy-evidence h3 { margin: 0 0 0.3rem; font-size: 0.85rem; }
.proxy-evidence p { margin: 0.3rem 0; }
</style>
