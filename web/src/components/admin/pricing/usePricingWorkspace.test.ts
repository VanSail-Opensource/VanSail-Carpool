import { effectScope, shallowRef } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { usePricingWorkspace } from './usePricingWorkspace'
const mocks = vi.hoisted(() => ({ dashboard: vi.fn(), health: vi.fn(), data: vi.fn() }))
vi.mock('@/api/pricingAnalysis', () => ({ pricingAnalysisApi: mocks }))

beforeEach(() => vi.clearAllMocks())
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })
it('loads one overview projection and keeps other analytical datasets deferred', async () => {
 mocks.dashboard.mockResolvedValue({ products: [], health: [], nextCursor: '' })
 mocks.health.mockResolvedValue({ health: [] })
 const mode = shallowRef('overview')
 const scope = effectScope()
 scope.run(() => usePricingWorkspace('', '', mode))
 await flushPromises()
 expect(mocks.dashboard).toHaveBeenCalledTimes(1)
 expect(mocks.data).not.toHaveBeenCalled()
 mode.value = 'user'
 await flushPromises()
 expect(mocks.dashboard).toHaveBeenCalledTimes(1)
 expect(mocks.health).not.toHaveBeenCalled()
 scope.stop()
})

it('refreshes product readiness after a minute and stops polling outside live routes', async () => {
 vi.useFakeTimers()
 vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
 mocks.dashboard.mockResolvedValue({ products: [], health: [], nextCursor: '' })
 mocks.health.mockResolvedValue({ health: [] })
 const mode = shallowRef('overview')
 const scope = effectScope()
 scope.run(() => usePricingWorkspace('', '', mode))
 await flushPromises()
 expect(mocks.dashboard).toHaveBeenCalledTimes(1)
 await vi.advanceTimersByTimeAsync(60_000)
 expect(mocks.dashboard).toHaveBeenCalledTimes(2)
 expect(mocks.health).toHaveBeenCalledTimes(2)
 mode.value = 'product'
 await flushPromises()
 await vi.advanceTimersByTimeAsync(60_000)
 expect(mocks.dashboard).toHaveBeenCalledTimes(2)
 expect(mocks.health).toHaveBeenCalledTimes(2)
 scope.stop()
})
