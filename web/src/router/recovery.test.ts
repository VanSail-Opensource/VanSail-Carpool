import { afterEach, describe, expect, it } from 'vitest'

import { beginRouteRecovery, completeRouteRecovery, isRouteChunkError, pendingRouteRecoveryPath } from './recovery'

describe('route recovery', () => {
  afterEach(() => sessionStorage.removeItem('txc-route-recovery'))

  it('allows one reload attempt for a failed route chunk', () => {
    expect(beginRouteRecovery('/catalog')).toBe(true)
    expect(pendingRouteRecoveryPath()).toBe('/catalog')
    expect(beginRouteRecovery('/catalog')).toBe(false)
  })

  it('clears recovery after the route succeeds', () => {
    expect(beginRouteRecovery('/activity')).toBe(true)
    completeRouteRecovery('/activity')
    expect(pendingRouteRecoveryPath()).toBeUndefined()
  })

  it('recognizes browser module loading failures only', () => {
    expect(isRouteChunkError(new TypeError('Failed to fetch dynamically imported module'))).toBe(true)
    expect(isRouteChunkError(new Error('API request failed'))).toBe(false)
  })

  it.each(['https://external.example/admin', '//external.example/admin', '/\\external.example/admin', '/admin\n/users'])('rejects unsafe recovery destinations: %s', path => {
    expect(beginRouteRecovery(path)).toBe(false)
    sessionStorage.setItem('txc-route-recovery', JSON.stringify({ path, attempts: 1 }))
    expect(pendingRouteRecoveryPath()).toBeUndefined()
  })
})
