import { computed, shallowRef, watch, nextTick, onMounted, onScopeDispose, type ShallowRef } from 'vue'
import type { IPLookupBGPTopology } from '@/api/ipLookup'
import { graphLayout, indexTopology, pathEdges, type GraphPreset } from './bgpGraph'
import { nodeWidth, nodeHeight, mergeSegments, type Segment } from './bgpLayout'
import { useGraphViewport } from './useGraphViewport'

export function useBGPGraph(topology: () => IPLookupBGPTopology, viewport: Readonly<ShallowRef<HTMLDivElement | null>>) {
  const preset = shallowRef<GraphPreset>('tier1')
  const selectedASN = shallowRef<number | null>(null)
  const selectedPath = shallowRef<number | null>(null)
  const size = shallowRef({ width: 320, height: 340 })
  const index = computed(() => indexTopology(topology()))
  const focusPath = computed(() => selectedPath.value === null ? undefined : topology().paths[selectedPath.value]?.asPath)
  const layout = computed(() => graphLayout(index.value, preset.value, focusPath.value))
  const view = useGraphViewport()
  const bounds = computed(() => ({ left: -view.offset.value.x / view.scale.value - 20, top: -view.offset.value.y / view.scale.value - 20, right: (size.value.width - view.offset.value.x) / view.scale.value + 20, bottom: (size.value.height - view.offset.value.y) / view.scale.value + 20 }))
  const visibleNodes = computed(() => layout.value.nodes.filter(node => node.x + nodeWidth >= bounds.value.left && node.x <= bounds.value.right && node.y + nodeHeight >= bounds.value.top && node.y <= bounds.value.bottom))
  const visibleEdges = computed(() => layout.value.edges.filter(edge => edge.bounds.right >= bounds.value.left && edge.bounds.left <= bounds.value.right && edge.bounds.bottom >= bounds.value.top && edge.bounds.top <= bounds.value.bottom))
  const highlighted = computed(() => {
    if (focusPath.value) return pathEdges(focusPath.value)
    if (selectedASN.value === null) return null
    const ids = new Set<string>()
    for (const path of topology().paths) if (path.asPath.includes(selectedASN.value)) for (const id of pathEdges(path.asPath)) ids.add(id)
    return ids
  })
  const selectedNode = computed(() => topology().nodes.find(node => node.asn === selectedASN.value))
  const edgeGroups = computed(() => {
    const groups = new Map<string, { id: string; relationship: string; state: string; d: string; count: number; segments: Segment[] }>()
    for (const edge of visibleEdges.value) {
      const state = highlighted.value ? highlighted.value.has(edge.id) ? 'highlighted' : 'dimmed' : 'normal'
      const id = `${edge.relationship}:${state}`
      const group = groups.get(id) ?? { id, relationship: edge.relationship, state, d: '', count: 0, segments: [] }
      if (view.scale.value < 0.15 && state !== 'highlighted') group.segments.push(...edge.segments)
      else group.d += `${edge.d} `
      group.count++; groups.set(id, group)
    }
    return [...groups.values()].map(group => ({ ...group, d: group.d + mergeSegments(group.segments) }))
  })
  function fit() { view.fit(layout.value.width, layout.value.height, size.value.width, size.value.height) }
  let observer: ResizeObserver | undefined
  onMounted(() => {
    const resize = () => { size.value = { width: viewport.value?.clientWidth ?? 320, height: viewport.value?.clientHeight ?? 340 }; if (!view.interacted.value) fit() }
    resize()
    if (viewport.value) { observer = new ResizeObserver(resize); observer.observe(viewport.value) }
  })
  onScopeDispose(() => observer?.disconnect())
  watch(() => `${topology().prefix}:${topology().origins.join(',')}`, () => {
    preset.value = 'tier1'; selectedASN.value = selectedPath.value = null; view.interacted.value = false
  })
  watch(() => layout.value.nodes.map(node => `${node.asn}:${node.x}:${node.y}`).join('|'), async () => { await nextTick(); if (!view.interacted.value) fit() })
  function select(asn: number, keyboard = false) {
    if (!keyboard && view.wasDragged()) return
    selectedASN.value = selectedASN.value === asn ? null : asn; selectedPath.value = null
  }
  function choosePreset(value: GraphPreset) { preset.value = value; selectedASN.value = selectedPath.value = null; view.interacted.value = false; fit() }
  function showPath(path: number) { selectedPath.value = selectedPath.value === path ? null : path; view.interacted.value = true; fit() }
  function keyboard(event: KeyboardEvent) { if (event.key === '0' && !(event.target as Element).closest('[data-asn]')) { event.preventDefault(); fit() } else view.keyboard(event) }
  return { ...view, viewportSize: size, preset, selectedASN, selectedPath, layout, visibleNodes, visibleEdges, edgeGroups, highlighted, selectedNode, fit, select, choosePreset, showPath, keyboard }
}
