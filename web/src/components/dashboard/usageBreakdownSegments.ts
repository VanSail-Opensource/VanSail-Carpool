import type { UsageBreakdown } from '@/api/usageBreakdown'
import { statisticsColors } from '@/components/statistics/statisticsFormat'
import { addUsageAmounts, belowOnePercent, readUsageAmount, usageAmount, usageShare, zeroUsageAmount, type ExactUsageAmount } from '@/utils/usageRational'

export type UsageBreakdownMetric = 'traffic' | 'cost'
export interface UsageBreakdownSegment {
  interactionId: string
  name: string
  kind: 'node' | 'others' | 'unallocated'
  amount: ExactUsageAmount
  color: string
  share: number
  percentage: number
}

export function usageNodeColor(nodeId: string): string {
  let hash = 2166136261
  for (const character of nodeId) hash = Math.imul(hash ^ character.charCodeAt(0), 16777619)
  return statisticsColors[(hash >>> 0) % statisticsColors.length]!
}

export function assignUsageNodeColors(nodeIDs: readonly string[], previous: ReadonlyMap<string, string> = new Map()): Map<string, string> {
  const active = [...new Set(nodeIDs)].sort()
  const result = new Map(previous)
  const used = new Set<string>()
  const assign: string[] = []
  for (const id of active) {
    const retained = previous.get(id)
    if (retained && !used.has(retained)) { result.set(id, retained); used.add(retained) }
    else assign.push(id)
  }
  for (const id of assign) {
    const base = statisticsColors.indexOf(usageNodeColor(id) as typeof statisticsColors[number])
    const color = statisticsColors.map((_, offset) => statisticsColors[(base + offset) % statisticsColors.length]!).find(value => !used.has(value)) ?? usageNodeColor(id)
    result.set(id, color); used.add(color)
  }
  // Retain recent identities across capped/refreshing windows without keeping
  // an unbounded account history in the mounted component.
  const current = new Set(active)
  for (const id of result.keys()) if (result.size > 256 && !current.has(id)) result.delete(id)
  return result
}

export function usageBreakdownSegments(history: UsageBreakdown, metric: UsageBreakdownMetric, retainedColors?: ReadonlyMap<string, string>) {
  const colors = retainedColors ?? assignUsageNodeColors(history.nodes.map(node => node.nodeId))
  const rows = history.nodes.map(node => ({
    interactionId: `node:${node.nodeId}`, name: node.name, kind: 'node' as const,
    amount: metric === 'cost' ? readUsageAmount(node.costMinor) : usageAmount(BigInt(node.uploadBytes) + BigInt(node.downloadBytes)),
    color: colors.get(node.nodeId) ?? usageNodeColor(node.nodeId),
  })).filter(row => row.amount.numerator > 0n).sort((a, b) => a.interactionId < b.interactionId ? -1 : a.interactionId > b.interactionId ? 1 : 0)
  const omitted = metric === 'cost' ? readUsageAmount(history.omitted.costMinor) : usageAmount(BigInt(history.omitted.uploadBytes) + BigInt(history.omitted.downloadBytes))
  const total = metric === 'cost' ? readUsageAmount(history.cost.knownTotalMinor) : rows.reduce((sum, row) => addUsageAmounts(sum, row.amount), omitted)
  let others = omitted
  const included = rows.filter(row => {
    if (!belowOnePercent(row.amount, total)) return true
    others = addUsageAmounts(others, row.amount)
    return false
  })
  const output: Omit<UsageBreakdownSegment, 'share' | 'percentage'>[] = included
  if (others.numerator > 0n) output.push({ interactionId: 'group:others', name: '', kind: 'others', amount: others, color: '#88929b' })
  const unallocated = metric === 'cost' ? readUsageAmount(history.cost.unallocatedMinor) : zeroUsageAmount()
  if (unallocated.numerator > 0n) output.push({ interactionId: 'group:unallocated', name: '', kind: 'unallocated', amount: unallocated, color: '#626a74' })
  const segments: UsageBreakdownSegment[] = output.map(row => {
    const share = usageShare(row.amount, total)
    return { ...row, share, percentage: share * 100 }
  })
  const positiveNodes = BigInt(history.positiveNodes[metric])
  return { total, segments, positiveNodes, showPie: positiveNodes > 1n && total.numerator > 0n }
}

export const usageSegmentID = (segment: UsageBreakdownSegment): string => segment.interactionId
export const usageSegmentValue = (segment: UsageBreakdownSegment): number => segment.share
export const usageSegmentColor = (segment: UsageBreakdownSegment): string => segment.color
