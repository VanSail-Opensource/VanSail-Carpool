import { afterEach, describe, expect, it, vi } from 'vitest'
import { publicSubscriptionMetadata } from './xnodeMember'

const token = 'a'.repeat(43)
const metadata = {
  username: 'member', url: '/subscription/' + token, usedBytes: '0', limitBytes: '0',
  expiresAt: null, active: false, incomplete: true, ready: false, firstSyncedAt: null, formats: {},
}
afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.useRealTimers()
})

describe('public subscription metadata transport', () => {
  it('omits session cookies and signatures and bypasses persistent caching', async () => {
    const payload = metadata
    const fetch = vi.fn<typeof globalThis.fetch>().mockResolvedValue(new Response(JSON.stringify(payload)))
    vi.stubGlobal('fetch', fetch)

    await expect(publicSubscriptionMetadata(token)).resolves.toEqual(payload)

    expect(fetch).toHaveBeenCalledTimes(1)
    const [url, options] = fetch.mock.calls[0]!
    expect(url).toBe('/api/v1/subscriptions/' + token + '/metadata')
    expect(options?.credentials).toBe('omit')
    expect(options?.cache).toBe('no-store')
    const headers = new Headers(options?.headers)
    expect([...headers.keys()].map(key => key.toLowerCase())).toEqual(['accept'])
    expect(headers.get('Accept')).toBe('application/json')
    expect(options?.signal).toBeInstanceOf(AbortSignal)
  })

  it.each([{ formats: null }, { formats: undefined }, { formats: [] }, { usedBytes: 100 }])('rejects malformed metadata before it enters page state: %j', async (invalid) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...metadata, ...invalid }))))
    await expect(publicSubscriptionMetadata(token)).rejects.toMatchObject({ status: 502, code: 'SUBSCRIPTION_UNAVAILABLE' })
  })

  it.each([401, 403, 404, 410, 503])('retains HTTP %s without expiring a browser session', async (status) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status })))
    const expired = vi.fn()
    const originalExperience = document.documentElement.dataset.experience
    document.documentElement.dataset.experience = 'browser'
    globalThis.addEventListener('txc:browser-session-expired', expired)
    try {
      await expect(publicSubscriptionMetadata(token)).rejects.toMatchObject({ status, code: 'SUBSCRIPTION_UNAVAILABLE' })
      expect(expired).not.toHaveBeenCalled()
    } finally {
      globalThis.removeEventListener('txc:browser-session-expired', expired)
      if (originalExperience === undefined) delete document.documentElement.dataset.experience
      else document.documentElement.dataset.experience = originalExperience
    }
  })

  it.each(['transport', 'body'])('bounds a stalled %s read', async (stage) => {
    vi.useFakeTimers()
    const response = new Response('{}')
    vi.spyOn(response, 'json').mockImplementation(() => new Promise(() => undefined))
    const fetch = vi.fn<typeof globalThis.fetch>().mockImplementation(() => stage === 'transport'
      ? new Promise<Response>(() => undefined) : Promise.resolve(response))
    vi.stubGlobal('fetch', fetch)
    const pending = publicSubscriptionMetadata(token)
    const rejected = expect(pending).rejects.toMatchObject({ name: 'TimeoutError' })

    await vi.advanceTimersByTimeAsync(30_000)
    await rejected

    expect(fetch.mock.calls[0]![1]?.signal?.aborted).toBe(true)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('propagates caller cancellation and clears its deadline', async () => {
    vi.useFakeTimers()
    const fetch = vi.fn<typeof globalThis.fetch>().mockImplementation(() => new Promise<Response>(() => undefined))
    vi.stubGlobal('fetch', fetch)
    const controller = new AbortController()
    const pending = publicSubscriptionMetadata(token, controller.signal)
    const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' })

    controller.abort()
    await rejected

    expect(fetch.mock.calls[0]![1]?.signal?.aborted).toBe(true)
    expect(vi.getTimerCount()).toBe(0)
  })
})
