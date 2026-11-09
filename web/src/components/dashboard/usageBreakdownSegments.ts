import type { UsageBreakdown } from '@/api/usageBreakdown'
import { statisticsColors } from '@/components/statistics/statisticsFormat'
import { addUsageAmounts, belowOnePercent, readUsageAmount, usageAmount, usageShare, zeroUsageAmount, type ExactUsageAmount } from '@/utils/usageRational'

export type UsageBreakdownMetric = 'traffic' | 'cost'
export interface UsageBreakdownSegment {
  interactionId: string
  name: string
  kind: 'node' | 'hourly' | 'others' | 'unallocated'
  amount: ExactUsageAmount
  color: string
  share: number
  percentage: number
}

export function assignUsageEntryColors(nodeIDs: readonly string[], previous: ReadonlyMap<string, string> = new Map()): Map<string, string> {
  const active = [...new Set(nodeIDs)].sort()
  const result = new Map(previous)
  const used = new Set<string>()
  const assign: string[] = []
  for (const id of active) {
    const retained = previous.get(id)
    if (retained && !used.has(retained)) { result.set(id, retained); used.add(retained) }
    else assign.push(id)
  }
  let cursor = 0
  for (const id of assign) {
    const { color, index } = availableUsageColor(cursor, used)
    result.set(id, color)
    used.add(color)
    cursor = index + 1
  }
  // Retain recent identities across capped/refreshing windows without keeping
  // an unbounded account history in the mounted component.
  const current = new Set(active)
  for (const id of result.keys()) if (result.size > 256 && !current.has(id)) result.delete(id)
  return result
}

// The eight curated colors are preferred; beyond them each identity still gets
// its own deterministic color instead of colliding with another squad.
function paletteUsageColor(index: number): string {
  if (index < statisticsColors.length) return statisticsColors[index]!
  const overflow = index - statisticsColors.length
  const hue = (overflow * 137.508 + 19) % 360
  const saturation = 46 + (overflow % 3) * 8
  const lightness = 58 + (Math.floor(overflow / 3) % 3) * 9
  return `hsl(${hue.toFixed(1)} ${saturation}% ${lightness}%)`
}

function availableUsageColor(start: number, used: ReadonlySet<string>): { color: string; index: number } {
  for (let index = Math.max(0, start); index < Math.max(0, start) + 4096; index++) {
    const color = paletteUsageColor(index)
    if (!used.has(color)) return { color, index }
  }
  return { color: paletteUsageColor(start), index: start }
}

export const usageEntryIDs = (history: UsageBreakdown): string[] => [...history.nodes.map(node => `node:${node.nodeId}`), ...history.hourlySquads.map(squad => `hourly:${squad.groupId}`)]
export function usageBreakdownSegments(history: UsageBreakdown, metric: UsageBreakdownMetric, retainedColors?: ReadonlyMap<string, string>) {
  const colors = assignUsageEntryColors(usageEntryIDs(history), retainedColors)
  const entries = [...history.nodes.map(node => ({
    interactionId: `node:${node.nodeId}`, name: node.name, kind: 'node' as const,
    amount: metric === 'cost' ? readUsageAmount(node.costMinor) : usageAmount(BigInt(node.uploadBytes) + BigInt(node.downloadBytes)),
  })), ...history.hourlySquads.map(squad => ({
    interactionId: `hourly:${squad.groupId}`, name: squad.name, kind: 'hourly' as const,
    amount: metric === 'cost' ? readUsageAmount(squad.costMinor) : usageAmount(BigInt(squad.uploadBytes) + BigInt(squad.downloadBytes)),
  }))]
  const rows = entries.map(row => ({ ...row, color: colors.get(row.interactionId)! })).filter(row => row.amount.numerator > 0n).sort((a, b) => a.interactionId < b.interactionId ? -1 : a.interactionId > b.interactionId ? 1 : 0)
  const omitted = metric === 'cost' ? readUsageAmount(history.omitted.costMinor) : usageAmount(BigInt(history.omitted.uploadBytes) + BigInt(history.omitted.downloadBytes))
  const total = metric === 'cost' ? readUsageAmount(history.cost.knownTotalMinor) : rows.reduce((sum, row) => addUsageAmounts(sum, row.amount), omitted)
  let others = omitted
  const included = rows.filter(row => {
    if (row.kind === 'hourly' || !belowOnePercent(row.amount, total)) return true
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
  const positiveEntries = BigInt(history.positiveEntries[metric])
  return { total, segments, positiveEntries, showPie: positiveEntries > 0n && total.numerator > 0n }
}

export const usageSegmentID = (segment: UsageBreakdownSegment): string => segment.interactionId
export const usageSegmentValue = (segment: UsageBreakdownSegment): number => segment.share
export const usageSegmentColor = (segment: UsageBreakdownSegment): string => segment.color
