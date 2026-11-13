import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Session } from '@/api/types'
import type { Turnstile } from '@/utils/turnstile'
import CaptchaGate from './CaptchaGate.vue'

const mocks = vi.hoisted(() => ({ verify: vi.fn(), load: vi.fn() }))
vi.mock('@/api/client', () => ({ api: { verifyCaptcha: mocks.verify } }))
vi.mock('@/utils/turnstile', () => ({ loadTurnstile: mocks.load }))
const challenge = { required: true, siteKey: 'constructed-key', action: 'txc_first_entry' as const }
const user: Session['user'] = { id: 'member-1', telegramId: '42', firstName: 'Member', lastName: '', telegramUsername: '', displayName: 'Member', username: null, role: 'user', onboardingState: 'intro', groupJoined: false, channelJoined: false, policyAcceptedAt: null, agreementRevision: 0, recoveryReason: '', createdAt: '2026-11-13T00:00:00Z', updatedAt: '2026-11-13T00:00:00Z' }
const verified: Session = { authenticated: true, user, captcha: { ...challenge, required: false } }
let callbacks: Parameters<Turnstile['render']>[1][]
let provider: Turnstile
beforeEach(() => {
  vi.resetAllMocks()
  callbacks = []
  provider = { render: vi.fn((_host, options) => { callbacks.push(options); return String(callbacks.length) }), remove: vi.fn(), reset: vi.fn() }
  mocks.load.mockResolvedValue(provider)
})
function deferred() {
  let resolve!: (value: Session) => void
  const promise = new Promise<Session>(done => { resolve = done })
  return { promise, resolve }
}

describe('captcha generation ownership', () => {
  it('rejects retired verification replies and provider callbacks after a challenge replacement', async () => {
    const pending = deferred()
    mocks.verify.mockReturnValueOnce(pending.promise).mockResolvedValueOnce(verified)
    const wrapper = mount(CaptchaGate, { props: { challenge } })
    await flushPromises()
    callbacks[0]!.callback('constructed-old-token')
    await wrapper.setProps({ challenge: { ...challenge, siteKey: 'replacement-key' } })
    await flushPromises()
    pending.resolve(verified)
    await flushPromises()
    callbacks[0]!['error-callback']()
    callbacks[0]!['expired-callback']()
    callbacks[0]!.callback('constructed-retired-token')
    await flushPromises()
    expect(wrapper.emitted('verified')).toBeUndefined()
    expect(provider.reset).not.toHaveBeenCalled()
    expect(mocks.verify).toHaveBeenCalledTimes(1)
    callbacks[1]!.callback('constructed-current-token')
    await flushPromises()
    expect(wrapper.emitted('verified')).toEqual([[verified]])
    wrapper.unmount()
  })

  it('cannot publish a session after the gate unmounts', async () => {
    const pending = deferred()
    mocks.verify.mockReturnValue(pending.promise)
    const wrapper = mount(CaptchaGate, { props: { challenge } })
    await flushPromises()
    callbacks[0]!.callback('constructed-token')
    wrapper.unmount()
    pending.resolve(verified)
    await flushPromises()
    expect(wrapper.emitted('verified')).toBeUndefined()
    expect(provider.remove).toHaveBeenCalledWith('1')
  })
})
