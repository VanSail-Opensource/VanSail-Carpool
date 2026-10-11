<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { IPLookupBGPTopology } from '@/api/ipLookup'
const props = defineProps<{ paths: IPLookupBGPTopology['paths']; selectedAsn: number | null; selectedPath: number | null; name: string }>()
defineEmits<{ select: [index: number] }>()
const page = shallowRef(1)
const filtered = computed(() => props.paths.map((path, index) => ({ ...path, index })).filter(path => props.selectedAsn === null || path.asPath.includes(props.selectedAsn)))
const pages = computed(() => Math.max(1, Math.ceil(filtered.value.length / 50)))
const visible = computed(() => filtered.value.slice((page.value - 1) * 50, page.value * 50))
watch(() => [props.selectedAsn, props.paths], () => { page.value = 1 })
</script>

<template>
  <details class="graph-routes" :open="selectedAsn !== null">
    <summary>{{ name || $t('ipLookupLive.graph.routes') }} <span>{{ filtered.length }}</span></summary>
    <ul><li v-for="path in visible" :key="path.index"><UButton color="neutral" variant="ghost" :aria-pressed="selectedPath === path.index" @click="$emit('select', path.index)"><code>{{ path.asPath.map(asn => `AS${asn}`).join(' → ') }}</code><small>{{ path.collector }}<span v-if="path.exchange">{{ $t('ipLookupLive.graph.exchangeContext', { name: path.exchange }) }}</span><time v-if="path.observedAt" :datetime="path.observedAt">{{ path.observedAt }}</time></small></UButton></li></ul>
    <nav v-if="pages > 1" :aria-label="$t('ipLookupLive.graph.pathPages')"><UButton color="neutral" variant="ghost" icon="i-ph-caret-left" :disabled="page === 1" :aria-label="$t('ipLookupLive.graph.previous')" @click="page--" /><span>{{ $t('ipLookupLive.graph.page', { page, total: pages }) }}</span><UButton color="neutral" variant="ghost" icon="i-ph-caret-right" :disabled="page === pages" :aria-label="$t('ipLookupLive.graph.next')" @click="page++" /></nav>
  </details>
</template>

<style scoped>
.graph-routes summary { font-size: 0.8rem; cursor: pointer; overflow-wrap: anywhere; padding-block: 0.5rem; }
.graph-routes summary span { color: var(--text-muted); margin-left: 0.3rem; }
.graph-routes ul { list-style: none; padding: 0; margin: 0.3rem 0; max-height: 280px; overflow-y: auto; }
.graph-routes button { display: block; width: 100%; text-align: left; padding: 0.6rem 0; border: 0; border-bottom: 1px solid var(--line); background: transparent; color: var(--text); cursor: pointer; }
.graph-routes code { display: block; font-size: 0.75rem; overflow-wrap: anywhere; }
.graph-routes small { display: flex; flex-wrap: wrap; gap: 0.25rem 0.5rem; margin-top: 0.25rem; color: var(--text-muted); font-size: 0.7rem; }
.graph-routes button[aria-pressed=true] code { color: var(--ui-info); }
.graph-routes nav { display: flex; align-items: center; justify-content: center; gap: 0.7rem; font-size: 0.75rem; color: var(--text-muted); }
</style>
