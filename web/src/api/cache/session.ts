// Response snapshots live only in this document's authenticated session.
const entries = new Map<string, string>()
const maximumBytes = 8 * 1024 * 1024
const maximumEntries = 256
let bytes = 0
let owner: string | null = null
let generation = 0
let identityGeneration = 0

export function cacheGeneration(): number { return generation }
// Identity changes retire authentication effects; ordinary writes only expire snapshots.
export function cacheIdentityGeneration(): number { return identityGeneration }
export function hasCacheSession(): boolean { return owner !== null }

export function removeResponseCache(key: string): void {
  bytes -= (entries.get(key)?.length ?? 0) * 2
  entries.delete(key)
}

export function clearResponseCache(): void {
  entries.clear()
  bytes = 0
  generation += 1
}

export function setCacheSession(identity: string | null): boolean {
  if (identity === owner) return false
  owner = identity
  identityGeneration += 1
  clearResponseCache()
  return true
}

export function readResponseCache<T>(key: string): T | undefined {
  const serialized = entries.get(key)
  if (serialized === undefined || owner === null) return undefined
  entries.delete(key)
  entries.set(key, serialized)
  try { return JSON.parse(serialized) as T }
  catch { removeResponseCache(key); return undefined }
}

export function writeResponseCache(key: string, value: unknown, expectedGeneration: number): void {
  if (owner === null || expectedGeneration !== generation || value === undefined) return
  let serialized: string | undefined
  try { serialized = JSON.stringify(value) }
  catch { removeResponseCache(key); return }
  if (serialized === undefined) return
  if (serialized.length * 2 > maximumBytes / 2) {
    removeResponseCache(key)
    return
  }
  bytes -= (entries.get(key)?.length ?? 0) * 2
  entries.delete(key)
  entries.set(key, serialized)
  bytes += serialized.length * 2
  while (entries.size > maximumEntries || bytes > maximumBytes) {
    const oldest = entries.keys().next().value
    if (oldest === undefined) break
    bytes -= entries.get(oldest)!.length * 2
    entries.delete(oldest)
  }
}
