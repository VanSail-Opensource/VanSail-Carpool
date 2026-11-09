const storageKey = 'txc-route-recovery'

interface RouteRecoveryRecord {
  path: string
  attempts: number
}

function localRoute(path: string): boolean {
  return path.startsWith('/') && !path.startsWith('//') && !path.includes('\\')
    && ![...path].some(character => character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127)
}

function routeStorage(): Storage | undefined {
  try {
    return globalThis.sessionStorage
  } catch {
    return undefined
  }
}

function readRecord(): RouteRecoveryRecord | undefined {
  const storage = routeStorage()
  if (!storage) return undefined
  try {
    const parsed = JSON.parse(storage.getItem(storageKey) ?? '') as Partial<RouteRecoveryRecord>
    if (typeof parsed.path !== 'string' || !localRoute(parsed.path) || typeof parsed.attempts !== 'number') return undefined
    return { path: parsed.path, attempts: parsed.attempts }
  } catch {
    return undefined
  }
}

export function pendingRouteRecoveryPath(): string | undefined {
  return readRecord()?.path
}

export function beginRouteRecovery(path: string): boolean {
  if (!localRoute(path)) return false
  const storage = routeStorage()
  if (!storage) return false
  const previous = readRecord()
  if (previous?.path === path && previous.attempts >= 1) return false
  const record: RouteRecoveryRecord = {
    path,
    attempts: previous?.path === path ? previous.attempts + 1 : 1,
  }
  try {
    storage.setItem(storageKey, JSON.stringify(record))
    return true
  } catch {
    return false
  }
}

export function completeRouteRecovery(path: string): void {
  const storage = routeStorage()
  if (!storage || readRecord()?.path !== path) return
  try {
    storage.removeItem(storageKey)
  } catch {
    // Storage can be unavailable in restricted WebViews.
  }
}

export function isRouteChunkError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error)
  return /chunkloaderror|loading chunk|dynamically imported module|importing a module script failed|failed to fetch dynamically imported module/i.test(message)
}
