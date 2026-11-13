import type { XNodeMetrics } from '@/api/xnode'

export function nodeStatus(node?: XNodeMetrics['nodes'][number]) {
  if (!node) return { label: 'xnode.statusUnavailable', color: 'neutral' as const }
  if (node.stale) return { label: 'xnode.stale', color: 'neutral' as const }
  if (!node.connected) return { label: 'xnode.offline', color: 'neutral' as const }
  if (!node.ready) return { label: 'xnode.notReady', color: 'warning' as const }
  if (!node.synchronized) return { label: 'xnode.degraded', color: 'warning' as const }
  return { label: 'xnode.online', color: 'success' as const }
}
