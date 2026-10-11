import { beforeEach, describe, expect, it, vi } from 'vitest'
const mocks = vi.hoisted(() => ({ response: vi.fn() }))
vi.mock('./http', async importOriginal => ({ ...await importOriginal<typeof import('./http')>(), requestResponse: mocks.response }))
import { streamIPDetails } from './ipLookupStream'

describe('signed live stream decoding', () => {
  beforeEach(() => vi.resetAllMocks())
  it('decodes split UTF-8 frames, publishes progress, and requires completion', async () => {
    const ip = '1.1.1.1'
    const text = [{ type: 'progress', ip, progress: { phase: 'topology', completed: 0, total: 8 } }, { type: 'section', ip, section: 'ptr', data: { domains: ['测试.example'] } }, { type: 'complete', ip, data: { ip }, progress: { phase: 'complete', completed: 8, total: 8 } }].map(event => `data: ${JSON.stringify(event)}\r\n\r\n`).join('')
    const bytes = new TextEncoder().encode(text)
    mocks.response.mockResolvedValue(new Response(new ReadableStream({ start(controller) { for (let i = 0; i < bytes.length; i += 7) controller.enqueue(bytes.slice(i, i + 7)); controller.close() } }), { headers: { 'Content-Type': 'text/event-stream' } }))
    const publish = vi.fn()
    expect(await streamIPDetails(ip, undefined, true, publish)).toEqual({ ip })
    expect(publish).toHaveBeenCalledTimes(3)
    expect(publish.mock.calls[1]![0].data.domains).toEqual(['测试.example'])
    expect(mocks.response.mock.calls[0]![1].query.refresh).toBe(true)
  })
  it('accepts JSON fallback and rejects mismatched addresses or truncated streams', async () => {
    mocks.response.mockResolvedValue(new Response('{"ip":"1.1.1.1"}', { headers: { 'Content-Type': 'application/json' } }))
    expect(await streamIPDetails('1.1.1.1', undefined, false, vi.fn())).toEqual({ ip: '1.1.1.1' })
    for (const frame of ['data: {"type":"progress","ip":"8.8.8.8"}\n\n', 'data: {"type":"progress","ip":"1.1.1.1"}\n\n']) {
      mocks.response.mockResolvedValue(new Response(frame, { headers: { 'Content-Type': 'text/event-stream' } }))
      await expect(streamIPDetails('1.1.1.1', undefined, false, vi.fn())).rejects.toThrow()
    }
  })
})
