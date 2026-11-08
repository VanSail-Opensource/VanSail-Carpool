import { afterEach, describe, expect, it, vi } from 'vitest'
import { usageBreakdownFixture } from '../../fixtures/usage-breakdown-data'

const request = vi.hoisted(() => vi.fn())
vi.mock('./http', async importOriginal => ({ ...await importOriginal<typeof import('./http')>(), request }))
import { usageBreakdownApi } from './usageBreakdown'

afterEach(() => vi.clearAllMocks())

describe('Usage Breakdown numeric response boundary', () => {
  it('accepts exact large/fractional fields through the owner-only no-store read', async () => {
    const value = usageBreakdownFixture(new URLSearchParams('large'))
    request.mockResolvedValue(value)
    const controller = new AbortController()
    expect(await usageBreakdownApi.get(controller.signal)).toBe(value)
    expect(request).toHaveBeenCalledWith('/api/v1/usage/breakdown', { signal: controller.signal, cache: 'no-store' })
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
