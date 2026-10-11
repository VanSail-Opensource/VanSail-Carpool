import type { components } from './generated'
import { ApiError, requestResponse } from './http'

type Details = components['schemas']['IPLookupLiveDetails']
export type LiveEvent = components['schemas']['IPLookupLiveEvent']

export async function streamIPDetails(ip: string, signal: AbortSignal | undefined, refresh: boolean, onEvent: (event: LiveEvent) => void): Promise<Details> {
  const streaming = typeof ReadableStream !== 'undefined'
  const response = await requestResponse('/api/v1/ip-lookup/details', {
    query: { ip, refresh }, signal, headers: { Accept: streaming ? 'text/event-stream' : 'application/json' },
  })
  if (!(response.headers.get('content-type') ?? '').includes('text/event-stream')) return await response.json() as Details
  const finish = (raw: string): Details | undefined => {
    const data = raw.split('\n').filter(line => line.startsWith('data:')).map(line => line.slice(5).trimStart()).join('\n')
    if (!data) return
    const event = JSON.parse(data) as LiveEvent
    if (event.ip !== ip) throw unavailable('IP_DETAILS_INVALID_RESPONSE')
    if (event.type === 'error') throw unavailable(event.errorCode)
    onEvent(event)
    if (event.type === 'complete') return event.data
  }
  let buffer = ''
  const consume = (chunk: string): Details | undefined => {
    buffer = (buffer + chunk).replace(/\r\n/g, '\n')
    if (buffer.length > 4 * 1024 * 1024) throw unavailable('IP_DETAILS_INVALID_RESPONSE')
    let boundary = buffer.indexOf('\n\n')
    while (boundary !== -1) {
      const frame = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      const result = finish(frame)
      if (result) return result
      boundary = buffer.indexOf('\n\n')
    }
  }
  const reader = response.body?.getReader?.()
  if (!reader) {
    const result = consume(await response.text())
    if (result) return result
    throw unavailable('IP_DETAILS_INVALID_RESPONSE')
  }
  const decoder = new TextDecoder()
  try {
    while (true) {
      const { value, done } = await reader.read()
      const result = consume(done ? decoder.decode() : decoder.decode(value, { stream: true }))
      if (result) return result
      if (done) break
    }
    throw unavailable('IP_DETAILS_UNAVAILABLE')
  } finally {
    await reader.cancel().catch(() => undefined)
    reader.releaseLock()
  }
}

function unavailable(code: string) { return new ApiError(502, { code, message: code }) }
