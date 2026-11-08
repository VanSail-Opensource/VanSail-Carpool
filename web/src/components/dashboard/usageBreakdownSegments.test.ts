import { describe, expect, it } from 'vitest'
import { usageBreakdownFixture } from '../../../fixtures/usage-breakdown-data'
import { assignUsageNodeColors, usageBreakdownSegments } from './usageBreakdownSegments'
import { addUsageAmounts, readUsageAmount, zeroUsageAmount } from '@/utils/usageRational'

describe('Usage Breakdown aggregation', () => {
  it.each(['', 'large', 'omitted', 'grouping', 'unallocated', 'unknown-term'])('conserves traffic and exact cost for %s', mode => {
    const view = usageBreakdownFixture(new URLSearchParams(mode))
    for (const metric of ['traffic', 'cost'] as const) {
      const plotted = usageBreakdownSegments(view, metric)
      const sum = plotted.segments.reduce((total, item) => addUsageAmounts(total, item.amount), zeroUsageAmount())
      expect(sum).toEqual(plotted.total)
      expect(plotted.segments.every(item => item.share >= 0 && item.share <= 1)).toBe(true)
    }
  })
  it('keeps >2^53 totals exact and never uses a rounded amount to allocate percentages', () => {
    const plotted = usageBreakdownSegments(usageBreakdownFixture(new URLSearchParams('large')), 'cost')
    expect(plotted.total).toEqual({ numerator: 27021597764222980n, denominator: 3n })
    expect(plotted.segments).toHaveLength(2)
  })
  it('uses pre-cap and pre-Others node counts separately for each metric', () => {
    const view = usageBreakdownFixture(new URLSearchParams('one-traffic'))
    expect(usageBreakdownSegments(view, 'traffic').showPie).toBe(false)
    expect(usageBreakdownSegments(view, 'cost').showPie).toBe(true)
    const omitted = usageBreakdownSegments(usageBreakdownFixture(new URLSearchParams('omitted')), 'traffic')
    expect(omitted.showPie).toBe(true)
    expect(omitted.positiveNodes).toBe(129n)
    expect(omitted.segments.map(row => row.kind)).toEqual(['node', 'others'])
  })
  it('retains unallocated cost without inventing a physical node or an empty pie', () => {
    const view = usageBreakdownFixture(new URLSearchParams('unallocated'))
    const plotted = usageBreakdownSegments(view, 'cost')
    expect(plotted.showPie).toBe(false)
    expect(plotted.segments).toMatchObject([{ kind: 'unallocated', amount: { numerator: 300n, denominator: 1n } }])
    expect(plotted.total).toEqual(readUsageAmount(view.cost.unallocatedMinor))
  })
  it('keeps the same node IDs and colors when Others membership changes across modes', () => {
    const view = usageBreakdownFixture(new URLSearchParams('grouping'))
    const traffic = usageBreakdownSegments(view, 'traffic')
    const cost = usageBreakdownSegments(view, 'cost')
    const primary = (rows: typeof traffic) => rows.segments.find(row => row.interactionId === 'node:major')
    expect(primary(traffic)?.color).toBe(primary(cost)?.color)
    expect(traffic.segments.find(row => row.kind === 'others')?.interactionId).toBe('group:others')
    expect(cost.segments.find(row => row.kind === 'others')?.interactionId).toBe('group:others')
    expect(traffic.segments.find(row => row.interactionId === 'node:threshold')).toBeDefined()
    expect(cost.segments.find(row => row.interactionId === 'node:threshold')).toBeDefined()
  })
  it('uses distinct available colors before metric filtering and retains them on refresh', () => {
    const view = usageBreakdownFixture()
    const initial = assignUsageNodeColors(view.nodes.map(node => node.nodeId))
    expect(new Set(initial.values()).size).toBe(3)
    const refreshed = assignUsageNodeColors(['added', ...view.nodes.map(node => node.nodeId).reverse()], initial)
    for (const [id, color] of initial) expect(refreshed.get(id)).toBe(color)
    const traffic = usageBreakdownSegments(view, 'traffic', refreshed)
    const cost = usageBreakdownSegments(view, 'cost', refreshed)
    for (const node of cost.segments.filter(node => node.kind === 'node')) expect(traffic.segments.find(row => row.interactionId === node.interactionId)?.color).toBe(node.color)
    let retained = initial
    for (let index = 0; index < 300; index++) retained = assignUsageNodeColors([`retired-${index}`], retained)
    expect(retained.size).toBeLessThanOrEqual(256)
  })
})
