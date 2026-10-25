import { effectScope, nextTick, shallowRef } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Catalog } from '@/api/types'
import { useCatalogAdmission } from './useCatalogAdmission'

afterEach(() => vi.useRealTimers())

describe('catalog admission', () => {
  it('releases at expiry and replaces server rejections after an authoritative reload', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-25T00:00:00Z'))
    const catalog = shallowRef<Catalog | null>(null)
    const refresh = vi.fn().mockResolvedValue(undefined)
    const scope = effectScope()
    const admission = scope.run(() => useCatalogAdmission(catalog, refresh))!
    catalog.value = { combos: [], addons: [], nodes: [], autoRenewalBlockedUntil: '2026-10-25T00:00:01Z' }
    await nextTick()
    expect(admission.blocked.value).toBe(true)
    await vi.advanceTimersByTimeAsync(1020)
    expect(admission.blocked.value).toBe(false)
    expect(refresh).toHaveBeenCalledOnce()
    admission.reject()
    expect(admission.blocked.value).toBe(true)
    catalog.value = { ...catalog.value, autoRenewalBlockedUntil: null }
    await nextTick()
    expect(admission.blocked.value).toBe(false)
    scope.stop()
  })

  it('cleans up an outstanding expiry refresh on disposal', async () => {
    vi.useFakeTimers()
    const catalog = shallowRef<Catalog | null>(null)
    const refresh = vi.fn()
    const scope = effectScope()
    scope.run(() => useCatalogAdmission(catalog, refresh))
    catalog.value = { combos: [], addons: [], nodes: [], autoRenewalBlockedUntil: new Date(Date.now() + 1000).toISOString() }
    await nextTick()
    scope.stop()
    await vi.advanceTimersByTimeAsync(2000)
    expect(refresh).not.toHaveBeenCalled()
  })
})
