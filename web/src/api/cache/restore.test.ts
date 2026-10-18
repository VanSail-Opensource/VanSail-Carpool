import { shallowRef } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { restoreCached, restoreItems, restoreRef } from './restore'
import { cacheGeneration, readResponseCache, setCacheSession, writeResponseCache } from './session'
import { responseCacheKey } from './policy'

beforeEach(() => { setCacheSession(null); setCacheSession('restore:member') })
afterEach(() => setCacheSession(null))

describe('compatible rendering snapshots', () => {
  it('discards outdated nested statistics without invoking the loader callback', () => {
    const key = responseCacheKey('/api/v1/admin/statistics')!
    writeResponseCache(key, { database: null }, cacheGeneration())
    const apply = vi.fn()
    expect(restoreCached('/api/v1/admin/statistics', apply)).toBe(false)
    expect(apply).not.toHaveBeenCalled()
    expect(readResponseCache(key)).toBeUndefined()
  })

  it('accepts empty collections and newly added optional response fields', () => {
    const key = responseCacheKey('/api/v1/catalog')!
    const next = { combos: [], addons: [], nodes: [], newMetadata: { revision: 2 } }
    writeResponseCache(key, next, cacheGeneration())
    const target = shallowRef<typeof next | null>(null)
    expect(restoreRef('/api/v1/catalog', target)).toBe(true)
    expect(target.value).toEqual(next)
  })

  it('contains an incompatible projection and keeps the caller state intact', () => {
    const key = responseCacheKey('/api/v1/legacy')!
    writeResponseCache(key, { items: null }, cacheGeneration())
    const target = shallowRef([{ id: 'current' }])
    expect(restoreItems('/api/v1/legacy', target)).toBe(false)
    expect(target.value).toEqual([{ id: 'current' }])
    expect(readResponseCache(key)).toBeUndefined()
  })
})
