import { afterEach, describe, expect, it, vi } from 'vitest'
import { usageBreakdownFixture } from '../../fixtures/usage-breakdown-data'

const request = vi.hoisted(() => vi.fn())
vi.mock('./http', async importOriginal => ({ ...await importOriginal<typeof import('./http')>(), request }))
import { usageBreakdownApi } from './usageBreakdown'

afterEach(() => vi.clearAllMocks())

describe('Usage Breakdown numeric response boundary', () => {
  it.each(['hourly', 'hourly-one', 'hourly-small', 'hourly-idle', 'hourly-ended', 'hourly-extra', 'hourly-omitted', 'hourly-large'])('accepts exact mixed/hourly history for %s', async mode => {
    const value = usageBreakdownFixture(new URLSearchParams(mode))
    request.mockResolvedValue(value)
    expect(await usageBreakdownApi.get()).toBe(value)
    if (mode === 'hourly-idle') expect(value.positiveEntries).toEqual({ traffic: '0', cost: '1' })
    if (mode === 'hourly-omitted') expect(value.positiveEntries).toEqual({ traffic: '129', cost: '129' })
  })
  it.each([
    ['duplicate squad', (value: ReturnType<typeof usageBreakdownFixture>) => { value.hourlySquads[1]!.groupId = value.hourlySquads[0]!.groupId }],
    ['empty squad identity', (value: ReturnType<typeof usageBreakdownFixture>) => { value.hourlySquads[0]!.groupId = '' }],
    ['negative hourly cost', (value: ReturnType<typeof usageBreakdownFixture>) => { value.hourlySquads[0]!.costMinor.numerator = '-1' }],
    ['incorrect recorded cost', (value: ReturnType<typeof usageBreakdownFixture>) => { value.hourlySquads[0]!.costMinor.numerator = '999' }],
    ['underreported combined count', (value: ReturnType<typeof usageBreakdownFixture>) => { value.positiveEntries.cost = '1' }],
    ['physical count above combined count', (value: ReturnType<typeof usageBreakdownFixture>) => { value.positiveNodes.traffic = '99' }],
    ['overreported combined count without omissions', (value: ReturnType<typeof usageBreakdownFixture>) => { value.positiveEntries.cost = '5' }],
    ['overreported physical count without omissions', (value: ReturnType<typeof usageBreakdownFixture>) => { value.positiveNodes.cost = '3' }],
  ] as const)('rejects %s in combined hourly history', async (_, invalidate) => {
    const value = usageBreakdownFixture(new URLSearchParams('hourly'))
    invalidate(value); request.mockResolvedValue(value)
    await expect(usageBreakdownApi.get()).rejects.toMatchObject({ code: 'API_RESPONSE_INCOMPATIBLE' })
  })
  it('rejects a fake positive entry count in an Unallocated-only response', async () => {
    const value = usageBreakdownFixture(new URLSearchParams('unallocated'))
    expect(value.nodes).toEqual([])
    expect(value.hourlySquads).toEqual([])
    value.positiveEntries.cost = '1'; request.mockResolvedValue(value)
    await expect(usageBreakdownApi.get()).rejects.toMatchObject({ code: 'API_RESPONSE_INCOMPATIBLE' })
  })
  it('enforces the combined 128-entry cap and requires disclosure for omitted totals', async () => {
    const value = usageBreakdownFixture(new URLSearchParams('hourly-omitted'))
    value.hourlySquads.push({ ...value.hourlySquads[0]!, groupId: 'beyond-limit' })
    request.mockResolvedValue(value)
    await expect(usageBreakdownApi.get()).rejects.toMatchObject({ code: 'API_RESPONSE_INCOMPATIBLE' })
    const hidden = usageBreakdownFixture(new URLSearchParams('hourly-omitted'))
    hidden.coverage.truncated = false; request.mockResolvedValue(hidden)
    await expect(usageBreakdownApi.get()).rejects.toMatchObject({ code: 'API_RESPONSE_INCOMPATIBLE' })
  })
  it('accepts exact large/fractional fields through the revalidated owner-only read', async () => {
    const value = usageBreakdownFixture(new URLSearchParams('large'))
    request.mockResolvedValue(value)
    const controller = new AbortController()
    expect(await usageBreakdownApi.get(controller.signal)).toBe(value)
    expect(request).toHaveBeenCalledWith('/api/v1/usage/breakdown', { signal: controller.signal, cache: 'no-cache' })
  })
  it.each([
    ['zero denominator', (value: ReturnType<typeof usageBreakdownFixture>) => { value.cost.knownTotalMinor.denominator = '0' }],
    ['negative cost', (value: ReturnType<typeof usageBreakdownFixture>) => { value.nodes[0]!.costMinor.numerator = '-1' }],
    ['not an integer', (value: ReturnType<typeof usageBreakdownFixture>) => { value.nodes[0]!.uploadBytes = 'NaN' }],
    ['negative count', (value: ReturnType<typeof usageBreakdownFixture>) => { value.positiveNodes.cost = '-1' }],
    ['oversized integer', (value: ReturnType<typeof usageBreakdownFixture>) => { value.omitted.downloadBytes = '9'.repeat(1025) }],
    ['duplicate node', (value: ReturnType<typeof usageBreakdownFixture>) => { value.nodes[1]!.nodeId = value.nodes[0]!.nodeId }],
    ['missing estimate', (value: ReturnType<typeof usageBreakdownFixture>) => { value.cost.legacy.amountMinor = null }],
    ['invalid coverage time', (value: ReturnType<typeof usageBreakdownFixture>) => { value.coverage.since = 'not-a-date' }],
    ['inconsistent allocated cost', (value: ReturnType<typeof usageBreakdownFixture>) => { value.nodes[0]!.costMinor.numerator = '999999' }],
    ['inconsistent charged total', (value: ReturnType<typeof usageBreakdownFixture>) => { value.cost.actualMinor = '301' }],
    ['underreported physical count', (value: ReturnType<typeof usageBreakdownFixture>) => { value.positiveNodes.traffic = '1' }],
  ] as const)('rejects %s before it reaches chart state', async (_, invalidate) => {
    const value = usageBreakdownFixture()
    invalidate(value)
    request.mockResolvedValue(value)
    await expect(usageBreakdownApi.get()).rejects.toMatchObject({ code: 'API_RESPONSE_INCOMPATIBLE' })
  })
  it('rejects malformed successful response shapes without a render exception', async () => {
    request.mockResolvedValue({ ...usageBreakdownFixture(), nodes: null })
    await expect(usageBreakdownApi.get()).rejects.toMatchObject({ code: 'API_RESPONSE_INCOMPATIBLE' })
  })
})
