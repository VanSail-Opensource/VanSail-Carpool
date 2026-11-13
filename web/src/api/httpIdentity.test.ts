import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearResponseCache, setCacheSession } from './cache/session'
import { onboardingRequiredEvent, request, requestBlob, requestResponse, requestStream } from './http'

const expiredEvent = 'txc:browser-session-expired'
let previousExperience: string | undefined
beforeEach(() => {
  previousExperience = document.documentElement.dataset.experience
  document.documentElement.dataset.experience = 'browser'
  setCacheSession('browser:old-owner')
})
afterEach(() => {
  setCacheSession(null)
  if (previousExperience === undefined) delete document.documentElement.dataset.experience
  else document.documentElement.dataset.experience = previousExperience
  vi.unstubAllGlobals()
})

const readers = { request, requestBlob, requestResponse, requestStream }
const cases = [
  [401, 'UNAUTHENTICATED', expiredEvent],
  [403, 'ADMIN_REQUIRED', expiredEvent],
  [403, 'ONBOARDING_REQUIRED', onboardingRequiredEvent],
  [403, 'CAPTCHA_REQUIRED', onboardingRequiredEvent],
] as const

describe('HTTP authentication side effects follow their request owner', () => {
  for (const [name, read] of Object.entries(readers)) {
    it.each(cases)(`${name} retires delayed %s %s notifications after owner changes`, async (status, code, event) => {
      let finish!: (response: Response) => void
      const fetch = vi.fn(() => new Promise<Response>(resolve => { finish = resolve }))
      vi.stubGlobal('fetch', fetch)
      const notified = vi.fn()
      window.addEventListener(event, notified)
      try {
        const pending = read('/api/v1/admin/users')
        const rejected = expect(pending).rejects.toMatchObject({ status, code })
        await vi.waitFor(() => expect(fetch).toHaveBeenCalledOnce())
        setCacheSession('browser:new-owner')
        finish(new Response(JSON.stringify({ code, message: 'old request rejected' }), { status, headers: { 'Content-Type': 'application/json' } }))
        await rejected
        expect(notified).not.toHaveBeenCalled()
      } finally { window.removeEventListener(event, notified) }
    })
  }

  it('keeps authentication failures effective after unrelated snapshot invalidation', async () => {
    let finish!: (response: Response) => void
    const fetch = vi.fn(() => new Promise<Response>(resolve => { finish = resolve }))
    vi.stubGlobal('fetch', fetch)
    const notified = vi.fn()
    window.addEventListener(expiredEvent, notified)
    try {
      const pending = request('/api/v1/admin/users')
      const rejected = expect(pending).rejects.toMatchObject({ status: 401 })
      await vi.waitFor(() => expect(fetch).toHaveBeenCalledOnce())
      clearResponseCache()
      finish(new Response(JSON.stringify({ code: 'UNAUTHENTICATED', message: 'expired' }), { status: 401, headers: { 'Content-Type': 'application/json' } }))
      await rejected
      expect(notified).toHaveBeenCalledOnce()
    } finally { window.removeEventListener(expiredEvent, notified) }
  })
})
