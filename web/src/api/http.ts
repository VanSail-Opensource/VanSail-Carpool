import type { ApiErrorBody } from './types'
import { applyRequestSignature, requestBodyBytes } from './request-signing'
import { cachedTransport } from './cache/transport'
import { clearResponseCache } from './cache/session'

export type QueryValue = string | number | boolean | readonly string[] | undefined
export const onboardingRequiredEvent = 'txc:onboarding-required'

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown
  query?: Record<string, QueryValue>
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

async function responseError(response: Response): Promise<ApiError> {
  const contentType = response.headers.get('content-type') ?? ''
  const payload = contentType.includes('application/json')
    ? await response.json() as unknown
    : await response.text()
  const body = typeof payload === 'object' && payload !== null
    ? payload as ApiErrorBody
    : { code: 'HTTP_ERROR', message: String(payload || response.statusText) }
  const error = new ApiError(response.status, body)
  if ((response.status === 401 || error.code === 'ADMIN_REQUIRED') && typeof document !== 'undefined' && document.documentElement.dataset.experience === 'browser' && typeof globalThis.dispatchEvent === 'function') {
    globalThis.dispatchEvent(new Event('txc:browser-session-expired'))
  }
  if ((error.code === 'ONBOARDING_REQUIRED' || error.code === 'CAPTCHA_REQUIRED') && typeof globalThis.dispatchEvent === 'function') {
    globalThis.dispatchEvent(new Event(onboardingRequiredEvent))
  }
  return error
}

async function responsePayload<T>(response: Response): Promise<T> {
  if (!response.ok) throw await responseError(response)
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
  const url = createUrl(path, options.query)

  // Live 2xx data is authoritative. Runtime OpenAPI-shape validation belongs
  // to optional rendering snapshots, where it can safely discard stale cache.
  // Rejecting a fresh response here breaks rolling frontend/backend deploys:
  // a temporarily skewed generated contract turns an otherwise usable API
  // response into a synthetic 502 before feature code can apply fallbacks.
  return cachedTransport(url, options, async () => responsePayload<T>(await send(path, options)))
}

// Backup candidates bypass body signing so the browser never materializes a
// multi-gigabyte multipart body. The server accepts this one exact route only.
export async function streamAdminBackupUpload<T>(body: FormData, idempotencyKey: string): Promise<T> {
  clearResponseCache()
  const response = await fetch('/api/v1/admin/backups/upload', {
    method: 'POST',
    body,
    credentials: 'include',
    headers: { Accept: 'application/json', 'Idempotency-Key': idempotencyKey },
  })
  clearResponseCache()
  return responsePayload<T>(response)
}

export async function requestBlob(path: string, options: RequestOptions = {}): Promise<Blob> {
  const response = await send(path, {
    ...options,
    headers: { Accept: 'application/octet-stream', ...Object.fromEntries(new Headers(options.headers)) },
  })
  if (!response.ok) throw await responseError(response)
  return response.blob()
}

// Streaming reads share the ordinary session, signatures and safe error handling.
export async function requestResponse(path: string, options: RequestOptions = {}): Promise<Response> {
  const response = await send(path, { ...options, cache: 'no-store' })
  if (!response.ok) throw await responseError(response)
  return response
}
