import { hmac } from '@noble/hashes/hmac.js'
import { sha256 } from '@noble/hashes/sha2.js'

import { BrowserCapabilityError, secureRandomBytes } from '@/utils/browserCompatibility'
import { isBrowserExperience } from '@/utils/experience'

const requestKeyCookie = 'txc_request_key'
const keyPattern = /^[A-Za-z0-9_-]{43}$/

function encodeText(value: string): Uint8Array {
  try {
    if (typeof TextEncoder !== 'function') throw new BrowserCapabilityError()
    return new TextEncoder().encode(value)
  } catch (caught) {
    if (caught instanceof BrowserCapabilityError) throw caught
    throw new BrowserCapabilityError()
  }
}

function readCookie(name: string): string | undefined {
  let cookies: string
  try {
    if (typeof document === 'undefined') return undefined
    cookies = document.cookie
  } catch {
    // Cookie access can be blocked by embedded/restricted WebView policies.
    return undefined
  }

  const prefix = `${name}=`
  for (const part of cookies.split(';')) {
    const value = part.trim()
    if (!value.startsWith(prefix)) continue
    try {
      const decoded = decodeURIComponent(value.slice(prefix.length))
      return decoded || undefined
    } catch {
      return undefined
    }
  }
  return undefined
}

function decodeBase64Url(value: string): Uint8Array {
  const standard = value.replace(/-/g, '+').replace(/_/g, '/')
  const padded = standard.padEnd(Math.ceil(standard.length / 4) * 4, '=')
  return Uint8Array.from(atob(padded), (character) => character.charCodeAt(0))
}

function encodeBase64Url(value: Uint8Array): string {
  let binary = ''
  for (const byte of value) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function hex(value: Uint8Array): string {
  return [...value].map((byte) => byte.toString(16).padStart(2, '0')).join('')
}

function buffer(value: Uint8Array): ArrayBuffer {
  return value.buffer.slice(value.byteOffset, value.byteOffset + value.byteLength) as ArrayBuffer
}

function webCrypto(): SubtleCrypto | undefined {
  try {
    const subtle = globalThis.crypto?.subtle
    return subtle && typeof subtle.digest === 'function' && typeof subtle.importKey === 'function' && typeof subtle.sign === 'function'
      ? subtle
      : undefined
  } catch {
    return undefined
  }
}

async function digest(value: Uint8Array): Promise<Uint8Array> {
  const subtle = webCrypto()
  if (subtle) {
    try {
      return new Uint8Array(await subtle.digest('SHA-256', buffer(value)))
    } catch {
      // Some Telegram WebViews expose Web Crypto but reject it in their context.
    }
  }
  return sha256(value)
}

async function sign(keyBytes: Uint8Array, value: Uint8Array): Promise<Uint8Array> {
  const subtle = webCrypto()
  if (subtle) {
    try {
      const key = await subtle.importKey('raw', buffer(keyBytes), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign'])
      return new Uint8Array(await subtle.sign('HMAC', key, buffer(value)))
    } catch {
      // The signed pure-JS fallback below keeps protected requests authenticated.
    }
  }
  return hmac(sha256, keyBytes, value)
}

function canonicalTarget(url: string): string {
  const parsed = new URL(url, window.location.origin)
  return `${parsed.pathname}${parsed.search}`
}

function signingRequired(target: string): boolean {
  return target !== '/api/v1/auth/telegram' && !target.startsWith('/api/v1/payments/return/')
}

export async function applyRequestSignature(
  headers: Headers,
  method: string,
  url: string,
  body: Uint8Array,
): Promise<void> {
  const target = canonicalTarget(url)
  if (isBrowserExperience()) headers.set('X-TXC-Session-Scope', 'browser')
  if (!signingRequired(target)) return
  const encodedKey = isBrowserExperience()
    ? readCookie('txc_admin_request_key') ?? (method.toUpperCase() === 'GET' && target === '/api/v1/auth/browser/session' ? readCookie(requestKeyCookie) : undefined)
    : readCookie(requestKeyCookie)
  if (!encodedKey || !keyPattern.test(encodedKey)) return

  const timestamp = Math.floor(Date.now() / 1000).toString()
  const nonceBytes = secureRandomBytes(16)
  const nonce = encodeBase64Url(nonceBytes)
  const bodyHash = hex(await digest(body))
  const canonical = [method.toUpperCase(), target, timestamp, nonce, bodyHash].join('\n')
  const signature = hex(await sign(decodeBase64Url(encodedKey), encodeText(canonical)))
  headers.set('X-TXC-Timestamp', timestamp)
  headers.set('X-TXC-Nonce', nonce)
  headers.set('X-TXC-Signature', signature)
}

export function requestBodyBytes(body?: string): Uint8Array {
  return encodeText(body ?? '')
}
