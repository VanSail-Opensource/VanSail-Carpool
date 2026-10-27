import { effectScope, shallowRef } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import { usePricingWorkspace } from './usePricingWorkspace'
const mocks = vi.hoisted(() => ({ dashboard: vi.fn(), health: vi.fn(), data: vi.fn() }))
vi.mock('@/api/pricingAnalysis', () => ({ pricingAnalysisApi: mocks }))
afterEach(() => vi.restoreAllMocks())
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
