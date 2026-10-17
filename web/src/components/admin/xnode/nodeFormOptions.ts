import type { XNode, XNodeConfiguration } from '@/api/xnode'

export function nodeInboundItems(nodeId: string | undefined, nodes: XNode[], configurations: XNodeConfiguration[]) {
  const document = configurations.find(item => item.id === nodes.find(node => node.id === nodeId)?.configId)?.document
  if (!Array.isArray(document?.inbounds)) return []
  return document.inbounds.flatMap(item => item && typeof item === 'object' && typeof item.tag === 'string' && !['tunnel', 'dokodemo-door'].includes(item.protocol) ? [{ value: item.tag, label: item.tag }] : [])
}

export function objectOption(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}
}
