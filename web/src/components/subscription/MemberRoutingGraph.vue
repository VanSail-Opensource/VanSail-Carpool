<script setup lang="ts">
import { computed, nextTick, onMounted, onScopeDispose, shallowRef, useId, watch } from 'vue'
import type { RoutingPath, RoutingView } from '@/api/xnodeMember'
import { nodeHeight, nodeWidth, placeNodes, routeEdges, type GraphEdge } from '@/components/ip-lookup/bgpLayout'
import { useGraphViewport } from '@/components/ip-lookup/useGraphViewport'
import { useI18n } from '@/i18n'

const props = defineProps<{ nodes: RoutingView['nodes']; paths: RoutingPath[]; trace: RoutingPath | null; selected: string }>()
const emit = defineEmits<{ select: [id: string] }>()
const { t } = useI18n()
const viewport = shallowRef<InstanceType<typeof globalThis.HTMLDivElement> | null>(null)
const narrow = shallowRef(globalThis.innerWidth < 600)
const view = useGraphViewport()
const markerId = useId()
const layout = computed(() => {
  const nodes = props.nodes.map((node, index) => ({ ...node, asn: index + 1 }))
  const ids = new Map(nodes.map(node => [node.id, node.asn]))
  const distance = new Map<number, number>(), frequency = new Map<number, number>(), links = new Map<string, GraphEdge>()
  for (const path of props.paths) path.nodes.forEach((id, index) => {
    const asn = ids.get(id); if (!asn) return
    distance.set(asn, Math.max(distance.get(asn) ?? 0, index)); frequency.set(asn, (frequency.get(asn) ?? 0) + 1)
    const to = ids.get(path.nodes[index + 1] ?? '')
    if (to) { const key = `${asn}-${to}`; links.set(key, { id: key, from: asn, to, relationship: 'path' }) }
  })
  const placed = placeNodes(nodes, [...links.values()], distance, frequency)
  if (narrow.value) {
    const columns = new Map<number, typeof placed.nodes>()
    for (const node of placed.nodes) { const column = columns.get(node.layer) ?? []; column.push(node); columns.set(node.layer, column) }
    for (const [layer, column] of columns) column.forEach((node, index) => { node.x = 32 + index * (nodeWidth + 24); node.y = 32 + layer * (nodeHeight + 40) })
    placed.width = Math.max(232, ...[...columns.values()].map(column => column.length * (nodeWidth + 24) + 40))
    placed.height = Math.max(240, ...placed.nodes.map(node => node.y + nodeHeight + 32))
  }
  const edges = routeEdges(placed.nodes, [...links.values()]).map(edge => {
    if (!narrow.value) return edge
    const from = placed.nodes.find(n => n.asn === edge.from)!, to = placed.nodes.find(n => n.asn === edge.to)!
    if (from.layer === to.layer) return edge
    const down = to.y > from.y, sx = from.x + nodeWidth / 2, tx = to.x + nodeWidth / 2
    const sy = from.y + (down ? nodeHeight : 0), ty = to.y + (down ? 0 : nodeHeight), mid = (sy + ty) / 2
    return { ...edge, d: `M ${sx} ${sy} C ${sx} ${mid}, ${tx} ${mid}, ${tx} ${ty}` }
  })
  return { ...placed, edges, ids }
})
const highlighted = computed(() => new Set(props.trace?.nodes.slice(0, -1).map((id, i) => `${layout.value.ids.get(id)}-${layout.value.ids.get(props.trace!.nodes[i + 1]!)}`) ?? []))
const transform = computed(() => `translate(${view.offset.value.x} ${view.offset.value.y}) scale(${view.scale.value})`)
function fit() { if (viewport.value) view.fit(layout.value.width + 32, layout.value.height, viewport.value.clientWidth, viewport.value.clientHeight) }
function select(id: string) { if (!view.wasDragged()) emit('select', id) }
let observer: InstanceType<typeof globalThis.ResizeObserver> | undefined
onMounted(() => { if (viewport.value) { observer = new globalThis.ResizeObserver(() => { narrow.value = (viewport.value?.clientWidth ?? 600) < 600; fit() }); observer.observe(viewport.value); fit() } })
watch(layout, () => void nextTick(fit))
onScopeDispose(() => observer?.disconnect())
</script>

<template>
  <div class="grid gap-2 min-w-0">
    <div class="flex justify-end"><UButton variant="ghost" color="neutral" icon="i-ph-arrows-out" :label="t('managedRouting.fit')" @click="fit" /></div>
    <div ref="viewport" class="routing-viewport" tabindex="0" role="group" :aria-label="t('managedRouting.graphHint')" @wheel="view.wheel" @pointerdown="view.pointerDown" @pointermove="view.pointerMove" @pointerup="view.pointerUp" @pointercancel="view.pointerUp" @keydown="view.keyboard">
      <svg class="w-full h-full" :aria-label="t('managedRouting.graph')" role="group">
        <defs><marker :id="markerId" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto" markerUnits="strokeWidth"><path d="M 0 0 L 8 4 L 0 8 Z" fill="var(--ui-text-dimmed)" /></marker></defs>
        <g :transform="transform">
          <path v-for="edge in layout.edges" :key="edge.id" :d="edge.d" fill="none" :marker-end="`url(#${markerId})`" :class="highlighted.has(edge.id) ? 'route-highlight' : 'route-edge'" />
          <g v-for="node in layout.nodes" :key="node.id" :transform="`translate(${node.x} ${node.y})`" :data-asn="node.asn" role="button" tabindex="0" :aria-label="t('managedRouting.editNode', { name: node.name })" :aria-pressed="selected === node.id" class="routing-node" @click="select(node.id)" @keydown.enter.prevent="emit('select', node.id)" @keydown.space.prevent="emit('select', node.id)">
            <rect :width="nodeWidth" :height="nodeHeight" rx="8" :class="selected === node.id || trace?.nodes.includes(node.id) ? 'node-selected' : 'node-rest'" />
            <text :x="nodeWidth / 2" :y="nodeHeight / 2 + 5" text-anchor="middle">{{ node.name.length > 22 ? `${node.name.slice(0, 20)}…` : node.name }}</text>
          </g>
        </g>
      </svg>
    </div>
    <p class="text-xs text-muted">{{ t('managedRouting.graphHint') }}</p>
  </div>
</template>

<style scoped>
.routing-viewport { height: 320px; overflow: hidden; touch-action: none; background: var(--ui-bg-elevated); border-radius: 10px; }
.routing-viewport:focus-visible, .routing-node:focus-visible { outline: 2px solid var(--ui-primary); outline-offset: 2px; }
.routing-node { cursor: pointer; }
.routing-node text { fill: var(--ui-text); font-size: 13px; pointer-events: none; }
.node-rest { fill: var(--ui-bg); stroke: var(--ui-border); }
.node-selected { fill: var(--ui-bg); stroke: var(--ui-primary); stroke-width: 2; }
.route-edge { stroke: var(--ui-text-dimmed); stroke-width: 1.5; }
.route-highlight { stroke: var(--ui-primary); stroke-width: 3; }
</style>
