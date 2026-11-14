import { createUrl, type RequestOptions } from '../http'
import { responseCacheKey } from './policy'
import { readResponseCache, removeResponseCache } from './session'
import { isResponseCompatible } from '../contracts/responses'
import type { ShallowRef } from 'vue'

// Only rendering loaders opt into snapshots; request() still resolves with live data.
export function restoreCached<T>(path: string, apply: (value: T) => void, options: RequestOptions = {}, maximumAgeMs?: number): boolean {
  const key = responseCacheKey(createUrl(path, options.query), options)
  if (!key) return false
  try {
    const value = readResponseCache<T>(key, maximumAgeMs)
    if (value === undefined) return false
    if (!isResponseCompatible(path, (options.method ?? 'GET').toUpperCase(), value)) {
      removeResponseCache(key)
      return false
    }
    apply(value)
    return true
  } catch {
    // A stale snapshot is optional; it must never prevent the live request.
    removeResponseCache(key)
    return false
  }
}

export function restoreRef<T>(path: string, target: ShallowRef<T>, options: RequestOptions = {}, maximumAgeMs?: number): boolean {
  return restoreCached<T>(path, value => { target.value = value }, options, maximumAgeMs)
}

export function restoreItems<T>(path: string, target: ShallowRef<T[]>, options: RequestOptions = {}): boolean {
  return restoreCached<{ items: T[] }>(path, value => {
    if (!Array.isArray(value?.items)) throw new TypeError('CACHE_ITEMS_INVALID')
    target.value = value.items
  }, options)
}
