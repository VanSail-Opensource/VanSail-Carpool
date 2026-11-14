import { describe, expect, it } from 'vitest'
import { usageBreakdownFixture } from '../../../fixtures/usage-breakdown-data'
import { assignUsageEntryColors, usageEntryIDs, usageBreakdownSegments } from './usageBreakdownSegments'
import { addUsageAmounts, readUsageAmount, zeroUsageAmount } from '@/utils/usageRational'

describe('Usage Breakdown aggregation', () => {
  it('ranks both measures by exact descending share with stable identity and color', () => {
    const view = usageBreakdownFixture(new URLSearchParams('hourly-large'))
    const colors = assignUsageEntryColors(usageEntryIDs(view))
    for (const metric of ['traffic', 'cost'] as const) {
      const { segments } = usageBreakdownSegments(view, metric, colors)
      for (let index = 1; index < segments.length; index++) {
        const previous = segments[index - 1]!, current = segments[index]!
        expect(previous.amount.numerator * current.amount.denominator >= current.amount.numerator * previous.amount.denominator).toBe(true)
      }
      for (const row of segments.filter(row => row.kind === 'node' || row.kind === 'hourly')) expect(row.color).toBe(colors.get(row.interactionId))
    }
  })
  it.each(['', 'large', 'omitted', 'grouping', 'unallocated', 'unknown-term', 'hourly', 'hourly-only', 'hourly-one', 'hourly-small', 'hourly-idle', 'hourly-ended', 'hourly-extra', 'hourly-omitted', 'hourly-large'])('conserves traffic and exact cost for %s', mode => {
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
  it('uses combined pre-cap entry counts and renders a single 100% attributed entry', () => {
    const view = usageBreakdownFixture(new URLSearchParams('one-traffic'))
    expect(usageBreakdownSegments(view, 'traffic').showPie).toBe(true)
    expect(usageBreakdownSegments(view, 'traffic').segments[0]?.share).toBe(1)
    expect(usageBreakdownSegments(view, 'cost').showPie).toBe(true)
    const omitted = usageBreakdownSegments(usageBreakdownFixture(new URLSearchParams('omitted')), 'traffic')
    expect(omitted.showPie).toBe(true)
    expect(omitted.positiveEntries).toBe(129n)
    expect(omitted.segments.map(row => row.kind)).toEqual(['node', 'others'])
  })
  it('retains named hourly slices below 1%, scoped identities and exact recorded costs', () => {
    const mixed = usageBreakdownFixture(new URLSearchParams('hourly'))
    const traffic = usageBreakdownSegments(mixed, 'traffic'), cost = usageBreakdownSegments(mixed, 'cost')
    expect(traffic.segments.find(row => row.interactionId === 'node:tokyo')).toBeDefined()
    expect(traffic.segments.find(row => row.interactionId === 'hourly:tokyo')).toBeDefined()
    expect(cost.segments.find(row => row.interactionId === 'hourly:tokyo')?.amount).toEqual({ numerator: 40n, denominator: 1n })
    const colors = traffic.segments.filter(row => row.kind === 'node' || row.kind === 'hourly').map(row => row.color)
    expect(new Set(colors).size).toBe(colors.length)
    for (const metric of ['traffic', 'cost'] as const) {
      const tiny = usageBreakdownSegments(usageBreakdownFixture(new URLSearchParams('hourly-small')), metric)
      expect(tiny.segments.find(row => row.interactionId === 'hourly:small')).toMatchObject({ name: 'Small but named hourly squad', kind: 'hourly' })
      expect(tiny.segments.find(row => row.interactionId === 'hourly:small')!.percentage).toBeLessThan(1)
    }
  })
  it('renders an idle paid squad as 100% cost while keeping zero traffic and shared extras separate', () => {
    const idle = usageBreakdownFixture(new URLSearchParams('hourly-idle'))
    expect(usageBreakdownSegments(idle, 'traffic')).toMatchObject({ showPie: false, positiveEntries: 0n, segments: [] })
    expect(usageBreakdownSegments(idle, 'cost')).toMatchObject({ showPie: true, positiveEntries: 1n, segments: [{ kind: 'hourly', share: 1 }] })
    const shared = usageBreakdownSegments(usageBreakdownFixture(new URLSearchParams('hourly-extra')), 'cost')
    expect(shared.segments.find(row => row.kind === 'unallocated')?.amount).toEqual({ numerator: 25n, denominator: 1n })
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
    const initial = assignUsageEntryColors(usageEntryIDs(view))
    expect(new Set(initial.values()).size).toBe(3)
    const refreshed = assignUsageEntryColors(['node:added', ...usageEntryIDs(view).reverse()], initial)
    for (const [id, color] of initial) expect(refreshed.get(id)).toBe(color)
    const traffic = usageBreakdownSegments(view, 'traffic', refreshed)
    const cost = usageBreakdownSegments(view, 'cost', refreshed)
    for (const node of cost.segments.filter(node => node.kind === 'node')) expect(traffic.segments.find(row => row.interactionId === node.interactionId)?.color).toBe(node.color)
    let retained = initial
    for (let index = 0; index < 300; index++) retained = assignUsageEntryColors([`retired-${index}`], retained)
    expect(retained.size).toBeLessThanOrEqual(256)
  })
})
