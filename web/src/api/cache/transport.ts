import type { RequestOptions } from '../http'
import { isReadRequest, responseCacheKey } from './policy'
import { cacheGeneration, clearResponseCache, hasCacheSession, removeResponseCache, writeResponseCache } from './session'

const pending = new Map<string, Promise<unknown>>()
const latest = new Map<string, symbol>()

export async function cachedTransport<T>(url: string, options: RequestOptions, send: () => Promise<T>): Promise<T> {
  const method = (options.method ?? 'GET').toUpperCase()
  const path = url.split('?')[0]!
  const mutation = !isReadRequest(path, method)
  if (mutation) clearResponseCache()
  const generation = cacheGeneration()
  const key = hasCacheSession() ? responseCacheKey(url, options) : null
  // A signalled/custom-header request retains its own cancellation and response contract.
  const pendingKey = key && !options.signal && !options.headers ? `${generation}:${key}` : null
  const existing = pendingKey ? pending.get(pendingKey) : undefined
  if (existing) return existing as Promise<T>
  const ticket = Symbol()
  if (key) latest.set(key, ticket)
  const work = (async () => {
    try {
      const value = await send()
      if (key && latest.get(key) === ticket) {
        // Dashboard carries a legacy bearer URL alongside display data. Never
        // retain that capability in a rendering snapshot; live results stay intact.
        const snapshot = path === '/api/v1/dashboard' && value && typeof value === 'object'
          ? { ...value, subscriptionUrl: null }
          : value
        writeResponseCache(key, snapshot, generation)
      }
      return value
    } catch (error) {
      const status = (error as { status?: number } | null)?.status
      if (generation === cacheGeneration()) {
        if (status === 401) clearResponseCache()
        else if (key && (status === 403 || status === 404)) removeResponseCache(key)
      }
      throw error
    } finally {
      if (mutation) clearResponseCache()
      if (key && latest.get(key) === ticket) latest.delete(key)
    }
  })()
  if (pendingKey) pending.set(pendingKey, work)
  try { return await work }
  finally { if (pendingKey && pending.get(pendingKey) === work) pending.delete(pendingKey) }
}
