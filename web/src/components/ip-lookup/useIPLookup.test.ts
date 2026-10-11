import { defineComponent, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { IPLookupCheck } from '@/api/ipLookup'
import { ApiError } from '@/api/http'

const mocks = vi.hoisted(() => ({ state: vi.fn(), progress: vi.fn(), quote: vi.fn(), submit: vi.fn(), check: vi.fn(), replace: vi.fn(), routeQuery: { check: undefined as string | undefined } }))
vi.mock('@/api/ipLookup', () => ({ ipLookupApi: mocks }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: mocks.routeQuery }), useRouter: () => ({ replace: mocks.replace }) }))
const account = reactive({ user: { id: 'member' } })
vi.mock('@/stores/session', () => ({ useSessionStore: () => account }))
import { useIPLookup } from './useIPLookup'

let state: ReturnType<typeof useIPLookup>
const Harness = defineComponent({ setup() { state = useIPLookup(); return () => null } })
const quote = { ip: '8.8.8.8', refresh: false, charge: { currency: 'TXB', minor: '250', display: '2.50 TXB' }, useQuota: false, cacheReportId: '', cachedReport: null, cacheMatch: 'none', token: 'quote' }

describe('IP Lookup request safety', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    account.user.id = 'member'
    mocks.routeQuery.check = undefined
    mocks.state.mockResolvedValue({ enabled: true, lookupFee: quote.charge, refreshFee: quote.charge, allowance: null })
    mocks.progress.mockResolvedValue(null)
    mocks.quote.mockResolvedValue(quote)
    mocks.replace.mockResolvedValue(undefined)
    mocks.check.mockResolvedValue({ operation: { status: 'succeeded' }, report: null })
  })
  afterEach(() => { vi.useRealTimers() })

  it('submits the signed combined refresh price and keeps the preview through a balance error', async () => {
    const cachedReport = { id: 'cached', ip: quote.ip, status: 'succeeded', checkedAt: 'date' }
    mocks.quote.mockImplementation(async (_ip, refresh) => ({ ...quote, refresh, expiresAt: Math.floor(Date.now() / 1000) + 120, cachedReport, cacheMatch: 'exact', cacheReportId: 'cached', charge: { ...quote.charge, minor: refresh ? '350' : '0' } }))
    mocks.submit.mockRejectedValueOnce(new ApiError(409, { code: 'INSUFFICIENT_BALANCE', message: 'balance' }))
    const wrapper = mount(Harness)
    await flushPromises()
    state.ip.value = quote.ip
    await vi.advanceTimersByTimeAsync(400)
    await state.submit(true)
    expect(mocks.submit.mock.calls[0]![0]).toMatchObject({ refresh: true, charge: { minor: '350' }, useQuota: false })
    expect(state.report.value).toEqual(cachedReport)
    expect(state.cacheMatch.value).toBe('exact')
    expect(state.errorCode.value).toBe('INSUFFICIENT_BALANCE')
    wrapper.unmount()
  })

  it('does not charge for loading, quoting or invalid addresses', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    state.ip.value = 'localhost'
    await vi.advanceTimersByTimeAsync(400)
    expect(mocks.quote).not.toHaveBeenCalled()
    expect(mocks.submit).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('reuses the idempotency key after an ambiguous submission response', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    state.ip.value = quote.ip
    await vi.advanceTimersByTimeAsync(400)
    mocks.submit.mockRejectedValueOnce(new Error('response lost')).mockResolvedValueOnce({ id: 'operation' })
    await state.submit()
    await state.submit()
    expect(mocks.submit).toHaveBeenCalledTimes(2)
    expect(mocks.submit.mock.calls[0]![1]).toBe(mocks.submit.mock.calls[1]![1])
    expect(mocks.replace).toHaveBeenCalledWith({ query: { check: 'operation' } })
    wrapper.unmount()
  })

  it('suppresses duplicate submissions while waiting for acceptance', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    state.ip.value = quote.ip
    await vi.advanceTimersByTimeAsync(400)
    let accept!: (value: { id: string }) => void
    mocks.submit.mockReturnValue(new Promise(resolve => { accept = resolve }))
    const first = state.submit()
    await state.submit()
    expect(mocks.submit).toHaveBeenCalledTimes(1)
    accept({ id: 'operation' })
    await first
    wrapper.unmount()
  })

  it('automatically displays subnet cache without submission, quota or refresh', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    const cachedReport = { id: 'cached', ip: '8.8.8.7', verdict: 'unsuitable' }
    mocks.quote.mockResolvedValue({ ...quote, cacheReportId: 'cached', cachedReport, cacheMatch: 'subnet', useQuota: false, charge: { ...quote.charge, minor: '0' } })
    state.ip.value = quote.ip
    await vi.advanceTimersByTimeAsync(400)
    expect(state.quote.value?.charge.minor).toBe('0')
    expect(state.quote.value?.useQuota).toBe(false)
    expect(state.report.value).toEqual(cachedReport)
    expect(state.requestedIP.value).toBe('8.8.8.8')
    expect(state.cacheMatch.value).toBe('subnet')
    expect(mocks.quote).toHaveBeenCalledTimes(2)
    expect(mocks.quote).toHaveBeenCalledWith(quote.ip, true)
    await state.submit()
    expect(mocks.submit).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('discards a previous IP quote when the input changes', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    let resolveQuote!: (value: object) => void
    mocks.quote.mockReturnValueOnce(new Promise(resolve => { resolveQuote = resolve }))
    state.ip.value = quote.ip
    await vi.advanceTimersByTimeAsync(400)
    state.ip.value = '1.1.1.1'
    await flushPromises()
    resolveQuote({ ...quote, cachedReport: { id: 'old' }, cacheMatch: 'exact' })
    await flushPromises()
    expect(state.report.value).toBeNull()
    expect(state.quote.value).toBeNull()
    wrapper.unmount()
  })

  it('discards old account responses even after switching back to the same account', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    let resolveQuote!: (value: object) => void
    mocks.quote.mockReturnValueOnce(new Promise(resolve => { resolveQuote = resolve }))
    state.ip.value = quote.ip
    await vi.advanceTimersByTimeAsync(400)
    account.user.id = 'other'
    await flushPromises()
    account.user.id = 'member'
    await flushPromises()
    resolveQuote({ ...quote, cachedReport: { id: 'old' }, cacheMatch: 'exact' })
    await flushPromises()
    expect(state.report.value).toBeNull()
    expect(state.quote.value).toBeNull()
    expect(state.ip.value).toBe('')
    wrapper.unmount()
  })

  it('preserves a fresh receipt when rehydration fills the blank address', async () => {
    const retained: IPLookupCheck = {
      operation: { id: 'saved-check', kind: 'ip_reputation_lookup', status: 'succeeded', errorCode: null, createdAt: '2026-10-10T00:00:00Z', updatedAt: '2026-10-10T00:00:00Z', completedAt: '2026-10-10T00:00:00Z' },
      report: {
        id: 'fresh-report', ip: quote.ip, status: 'succeeded', verdict: 'suitable', reasons: [],
        facts: { country: '', city: '', latitude: null, longitude: null, asn: '', asnName: '', networkType: 'residential' },
        sources: { networkType: 'ipapi' }, databases: [{ id: 'ipapi', status: 'success' }], refusals: [],
        maxmind: { ip_risk_snapshot: null, static_ip_score: null, user_count: null, user_type: null },
        checkedAt: '2026-10-10T00:00:00Z', policyVersion: 'residential-v3', parserVersion: 'residential-v3', refundRequired: false,
      },
      requestedIP: quote.ip, cacheMatch: 'none', cached: false, charge: { currency: 'TXB', minor: '0', display: '0.00 TXB' }, usedQuota: true, refunded: false,
    }
    mocks.routeQuery.check = retained.operation.id
    mocks.check.mockResolvedValue(retained)
    mocks.quote.mockResolvedValue({ ...quote, cacheReportId: retained.report!.id, cachedReport: retained.report, cacheMatch: 'exact', charge: { ...quote.charge, minor: '0' } })
    const wrapper = mount(Harness)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(400)
    expect(state.ip.value).toBe(quote.ip)
    expect(state.check.value).toEqual(retained)
    expect(state.report.value).toEqual(retained.report)
    expect(state.cacheMatch.value).toBe('none')
    expect(state.check.value?.usedQuota).toBe(true)
    expect(mocks.submit).not.toHaveBeenCalled()
    state.ip.value = '1.1.1.1'
    await flushPromises()
    expect(state.check.value).toBeNull()
    wrapper.unmount()
  })

  it('stops polling after the owning view is disposed', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    state.ip.value = quote.ip
    await vi.advanceTimersByTimeAsync(400)
    mocks.submit.mockResolvedValue({ id: 'operation' })
    mocks.check.mockResolvedValue({ operation: { status: 'processing' }, report: null })
    await state.submit()
    expect(mocks.check).toHaveBeenCalledTimes(1)
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(5000)
    expect(mocks.check).toHaveBeenCalledTimes(1)
  })
})
