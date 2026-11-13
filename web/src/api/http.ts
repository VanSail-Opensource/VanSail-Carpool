import type { ApiErrorBody } from './types'
import { applyRequestSignature, requestBodyBytes } from './request-signing'
import { cachedTransport } from './cache/transport'
import { cacheIdentityGeneration } from './cache/session'
import { withRequestDeadline } from './requestDeadline'

export type QueryValue = string | number | boolean | readonly string[] | undefined
export const onboardingRequiredEvent = 'txc:onboarding-required'

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown
  query?: Record<string, QueryValue>
  timeoutMilliseconds?: number
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details?: Record<string, string>
  readonly requestId?: string

  constructor(status: number, body: ApiErrorBody) {
    super(body.message)
    this.name = 'ApiError'
    this.status = status
    this.code = body.code
    this.details = body.details
    this.requestId = body.requestId
  }
}

export function createUrl(path: string, query?: Record<string, QueryValue>): string {
  if (!query) return path

  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (Array.isArray(value)) {
      for (const item of value) params.append(key, item)
    } else if (value !== undefined) {
      params.set(key, String(value))
    }
  }

  const encoded = params.toString()
  return encoded ? `${path}?${encoded}` : path
}

async function send(path: string, options: RequestOptions): Promise<Response> {
  const { body, query, ...init } = options
  delete init.timeoutMilliseconds
  const url = createUrl(path, query)
  const method = (init.method ?? 'GET').toUpperCase()
  const headers = new Headers(init.headers)
  headers.set('Accept', headers.get('Accept') ?? 'application/json')
  const isForm = typeof FormData !== 'undefined' && body instanceof FormData

  if (isForm) {
    const outgoing = new Request(url, { ...init, method, body, credentials: 'include', headers })
    const bytes = new Uint8Array(await outgoing.clone().arrayBuffer())
    await applyRequestSignature(outgoing.headers, method, url, bytes)
    return fetch(outgoing)
  }

  const serialized = body === undefined ? undefined : JSON.stringify(body)
  if (serialized !== undefined) headers.set('Content-Type', 'application/json')
  await applyRequestSignature(headers, method, url, requestBodyBytes(serialized))
  return fetch(url, { ...init, method, body: serialized, credentials: 'include', headers })
}

async function responseError(response: Response, identityGeneration: number): Promise<ApiError> {
  const contentType = response.headers.get('content-type') ?? ''
  const payload = contentType.includes('application/json')
    ? await response.json() as unknown
    : await response.text()
  const body = typeof payload === 'object' && payload !== null
    ? payload as ApiErrorBody
    : { code: 'HTTP_ERROR', message: String(payload || response.statusText) }
  const error = new ApiError(response.status, body)
  const currentIdentity = identityGeneration === cacheIdentityGeneration()
  if (currentIdentity && (response.status === 401 || error.code === 'ADMIN_REQUIRED') && typeof document !== 'undefined' && document.documentElement.dataset.experience === 'browser' && typeof globalThis.dispatchEvent === 'function') {
    globalThis.dispatchEvent(new Event('txc:browser-session-expired'))
  }
  if (currentIdentity && (error.code === 'ONBOARDING_REQUIRED' || error.code === 'CAPTCHA_REQUIRED') && typeof globalThis.dispatchEvent === 'function') {
    globalThis.dispatchEvent(new Event(onboardingRequiredEvent))
  }
  return error
}

async function responsePayload<T>(response: Response, identityGeneration: number): Promise<T> {
  if (!response.ok) throw await responseError(response, identityGeneration)
  if (response.status === 204) return undefined as T

  const contentType = response.headers.get('content-type') ?? ''
  const payload = contentType.includes('application/json')
    ? await response.json() as unknown
    : await response.text()

  if (typeof payload === 'object' && payload !== null && 'data' in payload) {
    return (payload as { data: T }).data
  }

  return payload as T
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  if (options.timeoutMilliseconds !== undefined) {
    const { timeoutMilliseconds, ...init } = options
    return withRequestDeadline(timeoutMilliseconds, options.signal, signal => request<T>(path, { ...init, signal }))
  }
  const url = createUrl(path, options.query)
  const identityGeneration = cacheIdentityGeneration()

  // Live 2xx data is authoritative. Runtime OpenAPI-shape validation belongs
  // to optional rendering snapshots, where it can safely discard stale cache.
  // Rejecting a fresh response here breaks rolling frontend/backend deploys:
  // a temporarily skewed generated contract turns an otherwise usable API
  // response into a synthetic 502 before feature code can apply fallbacks.
  return cachedTransport(url, options, async () => responsePayload<T>(await send(path, options), identityGeneration))
}


export async function requestBlob(path: string, options: RequestOptions = {}): Promise<Blob> {
  const identityGeneration = cacheIdentityGeneration()
  const response = await send(path, {
    ...options,
    headers: { Accept: 'application/octet-stream', ...Object.fromEntries(new Headers(options.headers)) },
  })
  if (!response.ok) throw await responseError(response, identityGeneration)
  return response.blob()
}

// Streaming consumers own cancellation and can pipe directly to a browser file.
export async function requestStream(path: string, options: RequestOptions = {}): Promise<Response> {
  const identityGeneration = cacheIdentityGeneration()
  const response = await send(path, options)
  if (!response.ok) throw await responseError(response, identityGeneration)
  return response
}

// Streaming reads share the ordinary session, signatures and safe error handling.
export async function requestResponse(path: string, options: RequestOptions = {}): Promise<Response> {
  const identityGeneration = cacheIdentityGeneration()
  const response = await send(path, { ...options, cache: 'no-store' })
  if (!response.ok) throw await responseError(response, identityGeneration)
  return response
}
