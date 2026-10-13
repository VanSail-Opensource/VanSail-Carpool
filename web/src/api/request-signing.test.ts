import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { applyRequestSignature, requestBodyBytes } from './request-signing'

const encoder = new TextEncoder()
let previousExperience: string | undefined

beforeEach(() => {
  previousExperience = document.documentElement.dataset.experience
  document.documentElement.dataset.experience = 'miniapp'
})

function base64Url(bytes: Uint8Array): string {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function hex(value: ArrayBuffer): string {
  return [...new Uint8Array(value)].map((byte) => byte.toString(16).padStart(2, '0')).join('')
}

function buffer(value: Uint8Array): ArrayBuffer {
  return value.buffer.slice(value.byteOffset, value.byteOffset + value.byteLength) as ArrayBuffer
}

afterEach(() => {
  document.cookie = 'txc_request_key=; Max-Age=0; Path=/'
  document.cookie = 'txc_admin_request_key=; Max-Age=0; Path=/'
  if (previousExperience === undefined) delete document.documentElement.dataset.experience
  else document.documentElement.dataset.experience = previousExperience
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

async function expectedSignature(keyBytes: Uint8Array, headers: Headers, target: string): Promise<string> {
  const bodyHash = hex(await crypto.subtle.digest('SHA-256', buffer(requestBodyBytes())))
  const canonical = ['GET', target, headers.get('X-TXC-Timestamp'), headers.get('X-TXC-Nonce'), bodyHash].join('\n')
  const key = await crypto.subtle.importKey('raw', buffer(keyBytes), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign'])
  return hex(await crypto.subtle.sign('HMAC', key, buffer(encoder.encode(canonical))))
}

describe('request signing', () => {
  it.each(['miniapp', 'browser'])('uses only the %s signing key when both experiences have cookies', async (experience) => {
    const memberKey = new Uint8Array(32).fill(11)
    const browserKey = new Uint8Array(32).fill(22)
    document.cookie = `txc_request_key=${base64Url(memberKey)}; Path=/`
    document.cookie = `txc_admin_request_key=${base64Url(browserKey)}; Path=/`
    document.documentElement.dataset.experience = experience
    const headers = new Headers()
    const target = experience === 'browser' ? '/api/v1/admin/users' : '/api/v1/dashboard'

    await applyRequestSignature(headers, 'GET', target, requestBodyBytes())

    expect(headers.get('X-TXC-Session-Scope')).toBe(experience === 'browser' ? 'browser' : null)
    expect(headers.get('X-TXC-Signature')).toBe(await expectedSignature(experience === 'browser' ? browserKey : memberKey, headers, target))
  })

  it('uses an existing Mini App key only to discover browser entrance identity', async () => {
    const memberKey = new Uint8Array(32).fill(11)
    document.cookie = `txc_request_key=${base64Url(memberKey)}; Path=/`
    document.documentElement.dataset.experience = 'browser'
    const headers = new Headers()
    const target = '/api/v1/auth/browser/session'

    await applyRequestSignature(headers, 'GET', target, requestBodyBytes())

    expect(headers.get('X-TXC-Session-Scope')).toBe('browser')
    expect(headers.get('X-TXC-Signature')).toBe(await expectedSignature(memberKey, headers, target))
  })

  it('never borrows a Mini App key for browser admin mutations', async () => {
    document.cookie = `txc_request_key=${base64Url(new Uint8Array(32).fill(11))}; Path=/`
    document.documentElement.dataset.experience = 'browser'
    const headers = new Headers()

    await applyRequestSignature(headers, 'POST', '/api/v1/admin/users/user-1/balance-adjustments', requestBodyBytes('{}'))

    expect(headers.get('X-TXC-Session-Scope')).toBe('browser')
    expect(headers.has('X-TXC-Signature')).toBe(false)
  })
  it('signs the exact method, target, timestamp, nonce, and body digest', async () => {
    const keyBytes = new Uint8Array(32).fill(7)
    document.cookie = `txc_request_key=${base64Url(keyBytes)}; Path=/`
    vi.spyOn(Date, 'now').mockReturnValue(1_786_320_000_000)
    const headers = new Headers()
    const body = requestBodyBytes('{"value":"two"}')

    await applyRequestSignature(headers, 'post', '/api/v1/example?q=one+two', body)

    const timestamp = headers.get('X-TXC-Timestamp') ?? ''
    const nonce = headers.get('X-TXC-Nonce') ?? ''
    const bodyHash = hex(await crypto.subtle.digest('SHA-256', buffer(body)))
    const canonical = ['POST', '/api/v1/example?q=one+two', timestamp, nonce, bodyHash].join('\n')
    const key = await crypto.subtle.importKey('raw', buffer(keyBytes), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign'])
    const expected = hex(await crypto.subtle.sign('HMAC', key, buffer(encoder.encode(canonical))))
    expect(timestamp).toBe('1786320000')
    expect(nonce).toMatch(/^[A-Za-z0-9_-]{22}$/)
    expect(headers.get('X-TXC-Signature')).toBe(expected)
  })

  it('leaves the Telegram authentication bootstrap unsigned', async () => {
    document.cookie = `txc_request_key=${base64Url(new Uint8Array(32))}; Path=/`
    const headers = new Headers()
    await applyRequestSignature(headers, 'POST', '/api/v1/auth/telegram', requestBodyBytes('{}'))
    expect(headers.has('X-TXC-Signature')).toBe(false)
  })

  it('leaves the provider-return status projection unsigned', async () => {
    document.cookie = `txc_request_key=${base64Url(new Uint8Array(32))}; Path=/`
    const headers = new Headers()
    await applyRequestSignature(headers, 'GET', '/api/v1/payments/return/ezpay/payment-1/status?capability=test', requestBodyBytes())
    expect(headers.has('X-TXC-Signature')).toBe(false)
  })

  it('skips signing when cookie access is blocked by the WebView', async () => {
    vi.spyOn(document, 'cookie', 'get').mockImplementation(() => {
      throw new Error('cookie access blocked')
    })
    const headers = new Headers()

    await expect(applyRequestSignature(headers, 'POST', '/api/v1/example', requestBodyBytes('{}')))
      .resolves.toBeUndefined()

    expect(headers.has('X-TXC-Signature')).toBe(false)
  })

  it('keeps protected requests signed without Web Crypto subtle', async () => {
    const nativeCrypto = globalThis.crypto
    vi.stubGlobal('crypto', { getRandomValues: nativeCrypto.getRandomValues.bind(nativeCrypto) })
    document.cookie = `txc_request_key=${base64Url(new Uint8Array(32).fill(5))}; Path=/`
    const headers = new Headers()

    await applyRequestSignature(headers, 'POST', '/api/v1/example', requestBodyBytes('{}'))

    expect(headers.get('X-TXC-Nonce')).toMatch(/^[A-Za-z0-9_-]{22}$/)
    expect(headers.get('X-TXC-Signature')).toMatch(/^[a-f0-9]{64}$/)
  })

  it('fails closed when secure randomness is unavailable', async () => {
    vi.stubGlobal('crypto', {})
    document.cookie = `txc_request_key=${base64Url(new Uint8Array(32).fill(5))}; Path=/`

    await expect(applyRequestSignature(new Headers(), 'POST', '/api/v1/example', requestBodyBytes('{}')))
      .rejects.toMatchObject({ code: 'BROWSER_CAPABILITY_UNSUPPORTED' })
  })
})
