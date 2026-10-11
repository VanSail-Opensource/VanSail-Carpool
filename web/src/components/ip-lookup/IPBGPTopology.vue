<script setup lang="ts">
import { computed, useTemplateRef } from 'vue'
import type { IPLookupBGPTopology } from '@/api/ipLookup'
import LiveSourceNote from './LiveSourceNote.vue'
import IPBGPFocus from './IPBGPFocus.vue'
import { nodeWidth, nodeHeight } from './bgpLayout'
import { useBGPGraph } from './useBGPGraph'
const props = defineProps<{ topology: IPLookupBGPTopology }>()
const viewport = useTemplateRef<globalThis.HTMLDivElement>('viewport')
const { scale, offset, viewportSize, selectedASN, selectedRoute, focusedRoutes, layout, visibleNodes, edgeGroups, visibleEdges, fit, select, focusRoute, wheel, pointerDown, pointerMove, pointerUp, keyboard } = useBGPGraph(() => props.topology, viewport)
const showASNs = computed(() => scale.value >= 0.15)
const showNames = computed(() => scale.value >= 0.8)
</script>

<template>
  <section class="bgp-topology">
    <header><h3>{{ $t('ipLookupLive.bgpTitle') }}</h3><code v-if="topology.prefix">{{ topology.prefix }}</code></header>
    <template v-if="topology.nodes.length">
      <div class="graph-toolbar">
        <div class="graph-legend"><span v-for="relation in ['upstream', 'peer', 'customer', 'observed', 'path']" :key="relation" :class="`legend-${relation}`">{{ $t(`ipLookupLive.graph.relationship.${relation}`) }}</span></div>
        <span>{{ $t('ipLookupLive.graph.count', { shown: layout.nodes.length, total: topology.nodes.length }) }}</span>
        <UButton color="neutral" variant="ghost" icon="i-ph-corners-out" :aria-label="$t('ipLookupLive.graph.reset')" :title="$t('ipLookupLive.graph.reset')" @click="fit" />
      </div>
      <div ref="viewport" class="graph-viewport" tabindex="0" role="region" :aria-label="$t('ipLookupLive.graph.instructions')" @wheel="wheel" @pointerdown="pointerDown" @pointermove="pointerMove" @pointerup="pointerUp" @pointercancel="pointerUp" @lostpointercapture="pointerUp" @keydown="keyboard">
        <svg :viewBox="`0 0 ${viewportSize.width} ${viewportSize.height}`" :aria-label="$t('ipLookupLive.bgpTitle')">
          <g class="graph-scene" :transform="`translate(${offset.x}, ${offset.y}) scale(${scale})`">
            <g class="graph-edges"><path v-for="group in edgeGroups" :key="group.id" :d="group.d" :data-edge-count="group.count" :class="[`edge-${group.relationship}`, `edge-${group.state}`]"><title>{{ $t(`ipLookupLive.graph.relationship.${group.relationship}`) }}</title></path></g>
            <path v-for="edge in visibleEdges.filter(item => item.hops)" :key="edge.id" :d="edge.d" class="graph-path-hit"><title>{{ $t('ipLookupLive.graph.hops', { hops: edge.hops ?? 0 }) }}</title></path>
            <g v-for="node in visibleNodes" :key="node.asn" :data-asn="node.asn" :transform="`translate(${node.x}, ${node.y})`" class="graph-node" :class="{ 'graph-origin': node.layer === 0, 'graph-selected': node.asn === selectedASN, 'graph-tier1': node.tier1 }" role="button" tabindex="0" :aria-label="`AS${node.asn} ${node.name}`" @click="select(node.asn)" @keydown.enter.stop="select(node.asn, true)" @keydown.space.stop.prevent="select(node.asn, true)">
              <title>{{ `AS${node.asn} ${node.name}` }}</title><rect :width="nodeWidth" :height="nodeHeight" :rx="showASNs ? 5 : 0" /><text v-if="showASNs" x="10" :y="showNames ? 17 : 27" :style="{ fontSize: `${Math.min(24, 12 / scale)}px` }">AS{{ node.asn }}<tspan v-if="node.tier1" class="graph-tier-label"> {{ $t('ipLookupLive.graph.tierBadge') }}</tspan></text><text v-if="showNames" x="10" y="33" class="graph-node-name">{{ node.name.length > 23 ? `${node.name.slice(0, 22)}…` : node.name || $t('ipLookupLive.graph.unknownName') }}</text>
            </g>
          </g>
        </svg>
      </div>
      <p v-if="layout.fallback" class="graph-note">{{ $t('ipLookupLive.graph.tier1Fallback') }}</p>
      <IPBGPFocus v-if="selectedASN !== null" :asn="selectedASN" :routes="focusedRoutes" :selected-route="selectedRoute" @focus="focusRoute" />
      <p v-if="layout.upstreamsEmpty" class="graph-note">{{ $t(`ipLookupLive.graph.upstreamsEmpty.${layout.upstreamsEmpty}`) }}</p>
      <p v-if="topology.truncated" class="graph-note">{{ $t('ipLookupLive.graph.truncated') }}</p>
      <div class="graph-provenance"><LiveSourceNote :meta="topology" /><LiveSourceNote v-if="topology.edges.length" :meta="topology.relationshipMeta" /><LiveSourceNote v-if="topology.classificationMeta" :meta="topology.classificationMeta" /></div>
      <details class="graph-note"><summary>{{ $t('ipLookupLive.about') }}</summary><p>{{ $t('ipLookupLive.graph.instructions') }}</p><div class="graph-legend"><span v-for="relation in ['upstream', 'peer', 'observed']" :key="relation" :class="`legend-${relation}`">{{ $t(`ipLookupLive.graph.relationship.${relation}`) }}</span></div><p>{{ $t('ipLookupLive.graph.context') }}</p><p>{{ $t('ipLookupLive.graph.tier1Context') }}</p></details>
    </template>
    <LiveSourceNote v-else :meta="topology" />
  </section>
</template>

<style scoped>
.bgp-topology { min-width: 0; padding-block: 0.4rem 0.8rem; }
.bgp-topology header { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0.4rem 0.7rem; }
.bgp-topology h3 { font-size: 0.9rem; margin: 0; }
.bgp-topology header code { font-size: 0.78rem; }
.graph-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 0.3rem 0.6rem; margin-block: 0.55rem; }
.graph-toolbar > span { color: var(--text-muted); font-size: 0.7rem; margin-left: auto; white-space: nowrap; }
.graph-viewport { height: 340px; overflow: hidden; position: relative; touch-action: none; overscroll-behavior: contain; background: var(--surface); border: 1px solid var(--line); border-radius: 6px; cursor: grab; }
.graph-viewport:active { cursor: grabbing; }
.graph-viewport:focus-visible { outline: 2px solid var(--ui-info); outline-offset: 2px; }
.graph-viewport svg { display: block; width: 100%; height: 100%; user-select: none; }
.graph-edges path { fill: none; stroke: var(--text-muted); stroke-width: 1; opacity: 0.35; }
.graph-edges .edge-upstream { stroke: var(--ui-info); opacity: 0.55; }
.graph-edges .edge-peer { stroke: var(--ui-success); }
.graph-edges .edge-observed { stroke-dasharray: 4 4; }
.graph-edges .edge-customer { stroke: var(--ui-warning); opacity: 0.5; }
.graph-edges .edge-path { stroke-dasharray: 8 3; opacity: 0.55; }
.graph-path-hit { fill: none; stroke: transparent; stroke-width: 12; pointer-events: stroke; }
.graph-edges .edge-highlighted { stroke-width: 2.5; opacity: 1; }
.graph-edges .edge-dimmed { opacity: 0.08; }
.graph-node { cursor: pointer; }
.graph-node rect { fill: var(--surface-elevated, var(--surface)); stroke: var(--line); }
.graph-tier1 rect { stroke: var(--text-muted); }
.graph-origin rect, .graph-selected rect, .graph-node:focus rect { stroke: var(--ui-info); stroke-width: 2; }
.graph-node text { fill: var(--text); font: 12px var(--font-mono, monospace); pointer-events: none; }
.graph-node .graph-node-name { fill: var(--text-muted); font-family: var(--font-body, sans-serif); font-size: 10px; }
.graph-tier-label { fill: var(--text-muted); font-size: 9px; }
.graph-note, .graph-legend { font-size: 0.72rem; color: var(--text-muted); line-height: 1.5; }
.graph-note summary { cursor: pointer; padding-block: 0.4rem; }
.graph-provenance, .graph-legend { display: flex; flex-wrap: wrap; gap: 0.2rem 0.65rem; }
.graph-legend span { border-bottom: 2px dashed var(--text-muted); padding-bottom: 0.15rem; }
.graph-legend .legend-upstream { border-bottom: 2px solid var(--ui-info); }
.graph-legend .legend-peer { border-bottom: 2px solid var(--ui-success); }
.graph-legend .legend-customer { border-bottom: 2px solid var(--ui-warning); }
.graph-legend .legend-path { border-bottom-style: dashed; }
@media (min-width: 900px) { .graph-viewport { height: 420px; } }
</style>
