import type { RoutingChoice, RoutingPath, RoutingView } from '@/api/xnodeMember'
import { endpointAddress } from './routingPresentation'

export interface RoutingTreeNode {
  id: string
  asn: number
  name: string
  subtitle: string
  layer: number
  nodeId?: string
  templateIds?: string[]
  excludedRules?: string[]
  highlighted?: boolean
}

/** Prefix sharing preserves endpoint identity and never joins separate paths. */
export function routingTree(nodes: RoutingView['nodes'], paths: RoutingPath[], choices: RoutingChoice[], endpoints: NonNullable<RoutingView['endpoints']>, templates: RoutingView['templates'], you: string, trace: RoutingPath | null) {
  const names = new Map(nodes.map(node => [node.id, node.name]))
  const rules = new Map(choices.map(choice => [choice.id, choice]))
  const hosts = new Map(endpoints.map(endpoint => [endpoint.id, endpoint]))
  const catalog = new Map(templates.map(template => [template.id, template]))
  const result: RoutingTreeNode[] = [{ id: 'root', asn: 1, name: you, subtitle: '', layer: 0 }]
  const byKey = new Map<string, RoutingTreeNode>([['root', result[0]!]])
  const edges = new Map<string, { id: string; from: number; to: number; relationship: 'path'; highlighted: boolean }>()
  const append = (parent: RoutingTreeNode, key: string, value: Omit<RoutingTreeNode, 'id' | 'asn' | 'layer'>, highlighted: boolean) => {
    let node = byKey.get(key)
    if (!node) { node = { ...value, id: key, asn: result.length + 1, layer: parent.layer + 1 }; byKey.set(key, node); result.push(node) }
    node.highlighted = highlighted || node.highlighted
    const id = `${parent.asn}-${node.asn}`, previous = edges.get(id)
    edges.set(id, { id, from: parent.asn, to: node.asn, relationship: 'path', highlighted: highlighted || !!previous?.highlighted })
    return node
  }
  // On failed/unavailable preview, show individual configured edges with the
  // error/draft status instead of inventing a validated end-to-end path.
  const displayed = paths.length ? paths : choices.map(choice => ({ nodes: [choice.inboundNodeId, choice.outboundNodeId], rules: [choice.id], excludedRules: [] }))
  for (const path of displayed) {
    let parent = result[0]!
    const highlighted = !!trace && path.nodes.join() === trace.nodes.join() && path.rules.join() === trace.rules.join()
    for (const [index, id] of path.nodes.entries()) {
      const rule = rules.get(path.rules[Math.max(0, index - 1)] ?? '')
      const endpointId = index === 0 ? rule?.inboundEndpointId : rule?.outboundEndpointId
      // Share the ingress host before rule paths diverge. Subsequent hops keep
      // their rule identity, so equal destinations never merge rule branches.
      parent = append(parent, JSON.stringify([parent.asn, id, endpointId, index === 0 ? undefined : rule?.id]), { name: names.get(id) ?? id, subtitle: endpointAddress(hosts.get(endpointId ?? '')), nodeId: id }, highlighted)
    }
    const templateIds = [...new Set(path.rules.map(id => rules.get(id)?.templateId).filter((id): id is string => !!id))]
    if (templateIds.length) append(parent, JSON.stringify([parent.asn, path.rules, path.excludedRules]), { name: templateIds.map(id => catalog.get(id)?.name ?? id).join(' ∩ '), subtitle: '', templateIds, excludedRules: path.excludedRules }, highlighted)
  }
  return { nodes: result, edges: [...edges.values()] }
}
