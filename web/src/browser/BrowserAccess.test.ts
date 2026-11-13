import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { ApiError } from '@/api/http'
import { setLocale } from '@/i18n'
import BrowserAccess from './BrowserAccess.vue'

const exchange = vi.hoisted(() => vi.fn())
const replace = vi.hoisted(() => vi.fn())
vi.mock('./session', () => ({ useBrowserSession: () => ({ exchange }) }))
const wrappers: VueWrapper[] = []
const previousUrl = location.pathname + location.search + location.hash
function entrance() {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/', component: { template: '<div />' } },
    { path: '/admin/settings', component: { template: '<div />' } },
  ] })
  vi.spyOn(router, 'replace').mockImplementation(replace)
  const wrapper = mount(BrowserAccess, { global: { plugins: [router], stubs: {
    BrowserPreferences: true,
    BrowserLoading: { props: ['error', 'failureTitle', 'status'], template: '<main><p>{{ error ? failureTitle : status }}</p><p>{{ error }}</p><slot v-if="error" /></main>' },
  } } })
  wrappers.push(wrapper)
  return wrapper
}
function retry(wrapper: VueWrapper) { return wrapper.findAll('button').find(button => button.text() === 'Try again') }
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(accept => { resolve = accept })
  return { promise, resolve }
}
beforeEach(() => {
  vi.resetAllMocks()
  setLocale('en')
  history.replaceState({}, '', '/admin/access#token=constructed-private-token')
})
afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  history.replaceState({}, '', previousUrl)
})

describe('one-use browser access recovery', () => {
  it.each([
    new ApiError(503, { code: 'BROWSER_SESSION_UNAVAILABLE', message: 'Unavailable' }),
    new ApiError(429, { code: 'RATE_LIMITED', message: 'Slow down' }),
    new TypeError('Network unavailable'),
  ])('retains a temporary failure only in memory until an explicit retry', async failure => {
    exchange.mockRejectedValueOnce(failure).mockResolvedValueOnce('/admin/settings')
    const wrapper = entrance()
    await flushPromises()
    expect(location.hash).toBe('')
    expect(wrapper.text()).not.toContain('constructed-private-token')
    expect(wrapper.text()).toContain('Sign-in is temporarily unavailable')
    expect(exchange).toHaveBeenCalledOnce()
    await retry(wrapper)!.trigger('click')
    await flushPromises()
    expect(exchange).toHaveBeenCalledTimes(2)
    expect(exchange.mock.calls.map(call => call[0])).toEqual(['constructed-private-token', 'constructed-private-token'])
    expect(replace).toHaveBeenCalledExactlyOnceWith('/admin/settings')
  })

  it('discards an invalid link and offers the entrance instead of another exchange', async () => {
    exchange.mockRejectedValueOnce(new ApiError(401, { code: 'BROWSER_LINK_INVALID', message: 'Invalid' }))
    const wrapper = entrance()
    await flushPromises()
    expect(wrapper.text()).toContain('Generate a new access link')
    expect(wrapper.text()).toContain('Back to entrance')
    expect(retry(wrapper)).toBeUndefined()
    expect(exchange).toHaveBeenCalledOnce()
  })

  it('keeps retries exclusive while exchange is pending', async () => {
    const pending = deferred<string>()
    exchange.mockRejectedValueOnce(new TypeError('Offline')).mockReturnValueOnce(pending.promise)
    const wrapper = entrance()
    await flushPromises()
    const button = retry(wrapper)!
    await Promise.all([button.trigger('click'), button.trigger('click')])
    expect(exchange).toHaveBeenCalledTimes(2)
    pending.resolve('/admin/settings')
    await flushPromises()
    expect(replace).toHaveBeenCalledOnce()
  })

  it('aborts a disposed entrance and ignores a delayed success', async () => {
    const pending = deferred<string>()
    exchange.mockReturnValueOnce(pending.promise)
    const wrapper = entrance()
    const signal = exchange.mock.calls[0]![1] as AbortSignal
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
    pending.resolve('/admin/settings')
    await flushPromises()
    expect(replace).not.toHaveBeenCalled()
  })
})
