<script setup lang="ts">
import { computed, nextTick, onMounted, onScopeDispose, shallowRef, useId, watch } from 'vue'
import type { RoutingChoice, RoutingPath, RoutingView } from '@/api/xnodeMember'
import { nodeHeight, nodeWidth, placeNodes, routeEdges } from '@/components/ip-lookup/bgpLayout'
import { fitGraph } from '@/components/ip-lookup/useGraphViewport'
import { useI18n } from '@/i18n'
import { routingTree, type RoutingTreeNode } from './routingTree'
import { selectorLabel } from './routingPresentation'
import MemberRoutingTemplateDetails from './MemberRoutingTemplateDetails.vue'

const props = defineProps<{ nodes: RoutingView['nodes']; paths: RoutingPath[]; trace: RoutingPath | null; selected: string; choices: RoutingChoice[]; endpoints: NonNullable<RoutingView['endpoints']>; templates: RoutingView['templates']; status: string }>()
const emit = defineEmits<{ select: [id: string] }>()
const { t } = useI18n()
const viewport = shallowRef<InstanceType<typeof globalThis.HTMLDivElement> | null>(null)
const narrow = shallowRef(globalThis.innerWidth < 600)
const view = shallowRef({ scale: 1, offset: { x: 0, y: 0 } })
const markerId = useId()
const layout = computed(() => {
  const tree = routingTree(props.nodes, props.paths, props.choices, props.endpoints, props.templates, t('managedRouting.you'), props.trace)
  const distance = new Map(tree.nodes.map(node => [node.asn, node.layer]))
  const placed = placeNodes(tree.nodes, tree.edges, distance, new Map())
  if (narrow.value) {
    const columns = new Map<number, typeof placed.nodes>()
    for (const node of placed.nodes) { const column = columns.get(node.layer) ?? []; column.push(node); columns.set(node.layer, column) }
    for (const [layer, column] of columns) column.forEach((node, index) => { node.x = 32 + index * (nodeWidth + 24); node.y = 32 + layer * (nodeHeight + 40) })
    placed.width = Math.max(232, ...[...columns.values()].map(column => column.length * (nodeWidth + 24) + 40))
    placed.height = Math.max(240, ...placed.nodes.map(node => node.y + nodeHeight + 32))
  }
  const edges = routeEdges(placed.nodes, tree.edges).map(edge => {
    if (!narrow.value) return edge
    const from = placed.nodes.find(n => n.asn === edge.from)!, to = placed.nodes.find(n => n.asn === edge.to)!
    if (from.layer === to.layer) return edge
    const down = to.y > from.y, sx = from.x + nodeWidth / 2, tx = to.x + nodeWidth / 2
    const sy = from.y + (down ? nodeHeight : 0), ty = to.y + (down ? 0 : nodeHeight), mid = (sy + ty) / 2
    return { ...edge, d: `M ${sx} ${sy} C ${sx} ${mid}, ${tx} ${mid}, ${tx} ${ty}` }
  })
  return { ...placed, edges, highlighted: new Set(tree.edges.filter(edge => edge.highlighted).map(edge => edge.id)) }
})
const detail = shallowRef<RoutingTreeNode | null>(null)
const detailOpen = computed({ get: () => detail.value !== null, set: value => { if (!value) detail.value = null } })
const detailTemplates = computed(() => props.templates.filter(template => detail.value?.templateIds?.includes(template.id)))
const excluded = computed(() => selectorLabel((detail.value?.excludedRules ?? []).flatMap(id => { const choice = props.choices.find(choice => choice.id === id); const template = props.templates.find(template => template.id === choice?.templateId); return template ? [template.match] : [] }), ''))
const statusColor = computed(() => ({ active: 'var(--ui-success)', draft: 'var(--ui-warning)', checking: 'var(--ui-warning)', error: 'var(--ui-error)', disabled: 'var(--ui-text-dimmed)' })[props.status] ?? 'var(--ui-text-dimmed)')
function label(node: RoutingTreeNode) { return node.templateIds ? t('managedRouting.viewTemplate', { name: node.name }) : node.nodeId ? t('managedRouting.editNode', { name: node.name }) : node.name }
const transform = computed(() => `translate(${view.value.offset.x} ${view.value.offset.y}) scale(${view.value.scale})`)
function fit() { if (viewport.value) view.value = fitGraph(layout.value.width + 32, layout.value.height, viewport.value.clientWidth, viewport.value.clientHeight) }
function select(node: RoutingTreeNode) { if (node.templateIds) detail.value = node; else if (node.nodeId) emit('select', node.nodeId) }
let observer: InstanceType<typeof globalThis.ResizeObserver> | undefined
onMounted(() => { if (viewport.value) { observer = new globalThis.ResizeObserver(() => { narrow.value = (viewport.value?.clientWidth ?? 600) < 600; fit() }); observer.observe(viewport.value); fit() } })
watch(layout, () => void nextTick(fit))
onScopeDispose(() => observer?.disconnect())
</script>

<template>
  <div class="grid gap-2 min-w-0">
    <div ref="viewport" class="routing-viewport" role="group" :aria-label="t('managedRouting.graphHint')">
      <svg class="w-full h-full" :aria-label="t('managedRouting.graph')" role="group">
        <defs><marker :id="markerId" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto" markerUnits="strokeWidth"><path d="M 0 0 L 8 4 L 0 8 Z" fill="var(--ui-text-dimmed)" /></marker></defs>
        <g :transform="transform">
          <path v-for="edge in layout.edges" :key="edge.id" :d="edge.d" fill="none" :marker-end="`url(#${markerId})`" :class="layout.highlighted.has(edge.id) ? 'route-highlight' : 'route-edge'" />
          <g v-for="node in layout.nodes" :key="node.id" :transform="`translate(${node.x} ${node.y})`" :data-asn="node.asn" :role="node.nodeId || node.templateIds ? 'button' : 'group'" :tabindex="node.nodeId || node.templateIds ? 0 : undefined" :aria-label="label(node)" :aria-pressed="node.nodeId ? selected === node.nodeId : undefined" class="routing-node" @click="select(node)" @keydown.enter.prevent="select(node)" @keydown.space.prevent="select(node)">
            <rect :width="nodeWidth" :height="nodeHeight" rx="8" :class="selected === node.nodeId || node.highlighted ? 'node-selected' : 'node-rest'" />
            <title>{{ node.name }}{{ node.templateIds ? ` · ${t(`managedRouting.routeState.${status}`)}` : '' }}</title>
            <circle v-if="node.templateIds" :cx="12" :cy="12" r="4" :fill="statusColor" :aria-label="t(`managedRouting.routeState.${status}`)" />
            <text :x="nodeWidth / 2" :y="node.subtitle ? 23 : nodeHeight / 2 + 5" text-anchor="middle">{{ node.name.length > 18 ? `${node.name.slice(0, 16)}…` : node.name }}</text>
            <text v-if="node.subtitle" :x="nodeWidth / 2" y="43" text-anchor="middle" class="node-host">{{ node.subtitle.length > 24 ? `${node.subtitle.slice(0, 22)}…` : node.subtitle }}</text>
          </g>
        </g>
      </svg>
    </div>
    <p class="text-xs text-muted">{{ t('managedRouting.treeHint') }}</p>
    <MemberRoutingTemplateDetails v-model:open="detailOpen" :templates="detailTemplates" :excluded="excluded" :status="status" />
  </div>
</template>

<style scoped>
.routing-viewport { height: 300px; overflow: hidden; touch-action: auto; background: transparent; }
.routing-viewport:focus-visible, .routing-node:focus-visible { outline: 2px solid var(--ui-primary); outline-offset: 2px; }
.routing-node { cursor: pointer; }
.routing-node text { fill: var(--ui-text); font-size: 13px; pointer-events: none; }
.routing-node .node-host { fill: var(--ui-text-muted); font-size: 10px; }
.node-rest { fill: var(--ui-bg); stroke: var(--ui-border); }
.node-selected { fill: var(--ui-bg); stroke: var(--ui-primary); stroke-width: 2; }
.route-edge { stroke: var(--ui-text-dimmed); stroke-width: 1.5; }
.route-highlight { stroke: var(--ui-primary); stroke-width: 3; }
</style>

<style scoped>
@media (max-width: 599px) { .routing-viewport { height: 300px; } .routing-node text { font-size: 18px; } .routing-node .node-host { font-size: 14px; } }
</style>
