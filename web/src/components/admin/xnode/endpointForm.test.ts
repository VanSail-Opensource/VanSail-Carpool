import { describe, expect, it } from 'vitest'
import { reactive } from 'vue'
import type { XNodeEndpoint, XNodeEndpointField } from '@/api/xnode'
import { endpointDraft, endpointOptionErrors, endpointPayload } from './endpointForm'

const field = (key: string, kind: XNodeEndpointField['kind'], minimum = 0, maximum = 65535): XNodeEndpointField => ({ key, kind, minimum, maximum, group: 'hysteria', unit: '', advanced: false })
describe('administrator endpoint drafts', () => {
  it('returns a plain cloneable API payload from reactive nested option drafts', () => {
    const draft = reactive(endpointDraft())
    draft.overrides = { clientOptions: { xhttpHeaders: [{ name: 'X-Test', value: 'client' }], xhttpXPaddingBytes: { min: 100, max: 500 }, hysteriaPorts: [{ min: 20000, max: 30000 }], downloadEnabled: true, downloadAddress: 'download.example', downloadXmuxMaxConcurrency: { min: 1, max: 4 } } }
    const fields = [field('xhttpHeaders', 'headers'), field('xhttpXPaddingBytes', 'range'), field('hysteriaPorts', 'port-ranges'), field('downloadEnabled', 'boolean'), field('downloadAddress', 'string'), field('downloadXmuxMaxConcurrency', 'range')]
    const payload = structuredClone(endpointPayload(draft, fields))
    expect(payload.overrides?.clientOptions).toEqual(draft.overrides.clientOptions)
    payload.overrides!.clientOptions!.xhttpHeaders![0]!.value = 'changed-copy'
    expect((draft.overrides.clientOptions as { xhttpHeaders: { value: string }[] }).xhttpHeaders[0]?.value).toBe('client')
  })
  it('retains incomplete numeric text while sending a validator-compatible draft', () => {
    const draft = endpointDraft()
    Object.assign(draft, { port: '' })
    expect(endpointPayload(draft).port).toBe(0)
    expect(draft.port).toBe('')
    Object.assign(draft, { port: 443.5 })
    expect(endpointPayload(draft).port).toBe(0)
  })
  it('preserves opaque legacy overrides while omitting derived metadata from writes', () => {
    const source: XNodeEndpoint = { id: 'endpoint', nodeId: 'node', inboundTag: 'inbound', name: 'Entry', address: 'proxy.example', port: 443, enabled: true, hidden: false, metadata: { protocol: 'vless', transport: 'xhttp', security: 'tls' }, overrides: { legacyHost: { internalSquads: { mode: 'INCLUDE', squads: ['group'] } }, clientOptions: { xhttpPath: '/saved' } } }
    const draft = endpointDraft(source)
    draft.overrides.legacyHost = { changed: true }
    expect(source.overrides?.legacyHost).toEqual({ internalSquads: { mode: 'INCLUDE', squads: ['group'] } })
    expect(endpointPayload(draft)).not.toHaveProperty('metadata')
    expect(endpointPayload(endpointDraft(source))).toMatchObject({ overrides: source.overrides })
  })
  it('submits only current applicable values and retains inactive drafts', () => {
    const draft = endpointDraft()
    draft.overrides = { opaque: true, clientOptions: { xhttpPath: '/up', downloadEnabled: false, downloadAddress: 'retained.invalid', wsPath: '/old' } }
    const fields = [field('xhttpPath', 'string'), field('downloadEnabled', 'boolean'), { ...field('downloadAddress', 'string'), dependencies: [{ key: 'downloadEnabled', values: ['true'] }] }]
    expect(endpointPayload(draft, fields).overrides).toEqual({ opaque: true, clientOptions: { xhttpPath: '/up', downloadEnabled: false } })
    expect(draft.overrides.clientOptions).toMatchObject({ downloadAddress: 'retained.invalid', wsPath: '/old' })
    ;(draft.overrides.clientOptions as Record<string, unknown>).downloadEnabled = true
    expect(endpointPayload(draft, fields).overrides?.clientOptions).toHaveProperty('downloadAddress', 'retained.invalid')
  })
  it('validates inclusive ports, explicit hopping disable, bounded intervals and signed XMUX values', () => {
    const fields = [field('hysteriaPorts', 'port-ranges', 1), field('hysteriaHopInterval', 'range', 5, 2147483647), field('xmuxHKeepAlivePeriod', 'integer', -2147483648, 2147483647)]
    expect(endpointOptionErrors(fields, { hysteriaPorts: [{ min: 1, max: 65535 }], hysteriaHopInterval: { min: 5, max: 10 }, xmuxHKeepAlivePeriod: -1 })).toEqual({})
    expect(endpointOptionErrors(fields, { hysteriaPorts: [] })).toEqual({})
    expect(endpointOptionErrors(fields, { hysteriaPorts: [], hysteriaHopInterval: 'unfinished' })).toEqual({})
    expect(endpointOptionErrors(fields, { hysteriaPorts: [{ min: 443, max: 443 }], hysteriaHopInterval: 'unfinished' })).toHaveProperty('hysteriaHopInterval')
    for (const value of [[{ min: 0, max: 1 }], [{ min: 100, max: 99 }], [{ min: 1, max: 65536 }], '20000-30000']) expect(endpointOptionErrors(fields, { hysteriaPorts: value })).toEqual({ hysteriaPorts: 'ports' })
    expect(endpointOptionErrors(fields, { hysteriaHopInterval: { min: 1, max: 5 } })).toHaveProperty('hysteriaHopInterval')
  })
  it('validates administrator header rows without adopting the member protected-header policy', () => {
    const fields = [field('xhttpHeaders', 'headers')]
    expect(endpointOptionErrors(fields, { xhttpHeaders: [{ name: 'Authorization', value: 'configured-by-admin' }] })).toEqual({})
    for (const rows of [[{ name: 'Host', value: 'use-host-field' }], [{ name: 'X-Test', value: '\r\nInjected: 1' }], [{ name: 'X-Test', value: '' }, { name: 'x-test', value: '' }]]) expect(endpointOptionErrors(fields, { xhttpHeaders: rows })).toHaveProperty('xhttpHeaders')
  })
})
