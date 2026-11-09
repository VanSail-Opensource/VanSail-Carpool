import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { installBrowserRouteRecovery } from './routeRecovery'

afterEach(() => { vi.restoreAllMocks(); sessionStorage.removeItem('txc-route-recovery') })

function fixture(failures = 1, error: Error = new TypeError('Failed to fetch dynamically imported module')) {
  let attempts = 0
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/', component: { render: () => null } },
    { path: '/admin/users/:id', component: () => {
      attempts++
      return attempts <= failures ? Promise.reject(error) : Promise.resolve({ render: () => null })
    } },
  ] })
  const replace = vi.fn()
  const recovery = installBrowserRouteRecovery(router, replace)
  return { router, replace, recovery, attempts: () => attempts }
}

describe('browser lazy route recovery', () => {
  it.each([true, false])('preserves the failed destination on initial=%s navigation', async initial => {
    const f = fixture()
    if (!initial) await f.router.push('/')
    const target = '/admin/users/member?tab=history#billing'
    await f.router.push(target).catch(() => undefined)
    expect(f.replace).toHaveBeenCalledExactlyOnceWith(target)
    expect(f.recovery.error.value).toBeInstanceOf(TypeError)
    await f.router.replace(target)
    expect(f.attempts()).toBe(2)
    expect(f.router.currentRoute.value.fullPath).toBe(target)
    expect(f.recovery.error.value).toBeNull()
    expect(sessionStorage.getItem('txc-route-recovery')).toBeNull()
  })

  it('stops document retries after a persistent chunk failure', async () => {
    const f = fixture(Infinity)
    const target = '/admin/users/member'
    await f.router.push(target).catch(() => undefined)
    await f.router.replace(target).catch(() => undefined)
    expect(f.replace).toHaveBeenCalledTimes(1)
    expect(f.recovery.target.value).toBe(target)
    expect(f.recovery.error.value).toBeInstanceOf(TypeError)
  })

  it('does not reload for route evaluation errors or unavailable session storage', async () => {
    const broken = fixture(Infinity, new ReferenceError('route initialization failed'))
    await broken.router.push('/admin/users/member').catch(() => undefined)
    expect(broken.replace).not.toHaveBeenCalled()
    expect(broken.recovery.error.value).toBeInstanceOf(ReferenceError)
    vi.spyOn(globalThis, 'sessionStorage', 'get').mockImplementation(() => { throw new Error('storage denied') })
    const blocked = fixture(Infinity)
    await blocked.router.push('/admin/users/member').catch(() => undefined)
    expect(blocked.replace).not.toHaveBeenCalled()
    expect(blocked.recovery.error.value).toBeInstanceOf(TypeError)
  })
})
