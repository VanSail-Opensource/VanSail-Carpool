import type { RationalMinor, UsageBreakdown } from './usageBreakdown'
import { addUsageAmounts, readUsageAmount, usageAmount, zeroUsageAmount, type ExactUsageAmount } from '@/utils/usageRational'

const unsigned = (value: string) => /^[0-9]{1,1024}$/.test(value)
const fraction = (value: RationalMinor) => unsigned(value.numerator) && /^[1-9][0-9]{0,1023}$/.test(value.denominator)

/** The generated rendering validator checks shapes; exact units need these
 * endpoint-specific numeric/identity checks before they enter chart state. */
export function hasValidUsageUnits(value: UsageBreakdown): boolean {
  const valid = Number.isFinite(Date.parse(value.from)) && Number.isFinite(Date.parse(value.to))
    && (value.coverage.since === null || Number.isFinite(Date.parse(value.coverage.since)))
    && unsigned(value.coverage.gapCount) && unsigned(value.positiveNodes.traffic) && unsigned(value.positiveNodes.cost)
    && value.nodes.length <= 128 && new Set(value.nodes.map(node => node.nodeId)).size === value.nodes.length
    && value.nodes.every(node => node.nodeId !== '' && unsigned(node.uploadBytes) && unsigned(node.downloadBytes) && fraction(node.costMinor))
    && unsigned(value.omitted.uploadBytes) && unsigned(value.omitted.downloadBytes) && fraction(value.omitted.costMinor)
    && unsigned(value.cost.actualMinor) && fraction(value.cost.knownTotalMinor) && fraction(value.cost.unallocatedMinor)
    && (value.cost.legacy.status === 'available' ? value.cost.legacy.amountMinor !== null && fraction(value.cost.legacy.amountMinor)
      : ['none', 'original_term_unknown'].includes(value.cost.legacy.status) && value.cost.legacy.amountMinor === null)
  if (!valid) return false
  const same = (a: ExactUsageAmount, b: ExactUsageAmount) => a.numerator === b.numerator && a.denominator === b.denominator
  const nodeCost = value.nodes.reduce((sum, node) => addUsageAmounts(sum, readUsageAmount(node.costMinor)), zeroUsageAmount())
  const attributed = addUsageAmounts(nodeCost, readUsageAmount(value.omitted.costMinor))
  const total = readUsageAmount(value.cost.knownTotalMinor)
  const legacy = value.cost.legacy.amountMinor ? readUsageAmount(value.cost.legacy.amountMinor) : zeroUsageAmount()
  const charged = addUsageAmounts(usageAmount(BigInt(value.cost.actualMinor)), legacy)
  if (!same(addUsageAmounts(attributed, readUsageAmount(value.cost.unallocatedMinor)), total) || !same(charged, total)) return false
  const trafficNodes = value.nodes.filter(node => BigInt(node.uploadBytes) + BigInt(node.downloadBytes) > 0n).length
  const costNodes = value.nodes.filter(node => BigInt(node.costMinor.numerator) > 0n).length
  const omittedTraffic = BigInt(value.omitted.uploadBytes) + BigInt(value.omitted.downloadBytes) > 0n ? 1n : 0n
  const omittedCost = BigInt(value.omitted.costMinor.numerator) > 0n ? 1n : 0n
  return BigInt(value.positiveNodes.traffic) >= BigInt(trafficNodes) + omittedTraffic
    && BigInt(value.positiveNodes.cost) >= BigInt(costNodes) + omittedCost
}
