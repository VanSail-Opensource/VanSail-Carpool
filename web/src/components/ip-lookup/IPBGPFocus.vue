<script setup lang="ts">
import { shallowRef, watch, computed } from 'vue'
import type { FocusRoute } from './bgpGraph'
const props = defineProps<{ asn: number; routes: FocusRoute[]; selectedRoute: string | null }>()
defineEmits<{ focus: [id: string] }>()
const page = shallowRef(1)
const pages = computed(() => Math.max(1, Math.ceil(props.routes.length / 25)))
const visible = computed(() => props.routes.slice((page.value - 1) * 25, page.value * 25))
watch(() => props.asn, () => { page.value = 1 })
</script>

<template>
  <section class="graph-focus">
    <h4>{{ $t('ipLookupLive.graph.focused', { asn }) }}</h4>
    <p>{{ $t('ipLookupLive.graph.focusHelp') }}</p>
    <p v-if="!routes.length">{{ $t('ipLookupLive.status.empty') }}</p>
    <div class="graph-focus-routes"><UButton v-for="route in visible" :key="route.id" color="neutral" variant="ghost" :aria-pressed="selectedRoute === route.id" @click="$emit('focus', route.id)"><code>{{ route.asns.map(value => `AS${value}`).join(' → ') }}</code><span v-if="route.hops">{{ $t('ipLookupLive.graph.hops', { hops: route.hops }) }}</span></UButton></div>
    <nav v-if="pages > 1" :aria-label="$t('ipLookupLive.graph.focused', { asn })"><UButton color="neutral" variant="ghost" icon="i-ph-caret-left" :disabled="page === 1" :aria-label="$t('ipLookupLive.graph.focusPrevious')" @click="page--" /><span>{{ page }}/{{ pages }}</span><UButton color="neutral" variant="ghost" icon="i-ph-caret-right" :disabled="page === pages" :aria-label="$t('ipLookupLive.graph.focusNext')" @click="page++" /></nav>
  </section>
</template>

<style scoped>
.graph-focus { margin-block: 0.75rem; }
.graph-focus h4, .graph-focus p { font-size: 0.78rem; margin: 0.4rem 0; }
.graph-focus-routes { display: grid; gap: 0.2rem; max-height: 220px; overflow-y: auto; }
.graph-focus-routes button { justify-content: space-between; flex-wrap: wrap; text-align: left; }
.graph-focus-routes code { font-size: 0.75rem; overflow-wrap: anywhere; }
.graph-focus-routes span, .graph-focus nav { font-size: 0.7rem; color: var(--text-muted); }
.graph-focus-routes button[aria-pressed=true] { color: var(--ui-info); }
.graph-focus nav { display: flex; align-items: center; justify-content: center; gap: 0.5rem; }
</style>
