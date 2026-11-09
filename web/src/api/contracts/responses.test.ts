import { describe, expect, it } from 'vitest'
import { isResponseCompatible } from './responses'

describe('rendering snapshot compatibility', () => {
  it('validates endpoint draft previews without accepting credentials or malformed compatibility facts', () => {
    const response = { valid: false, metadata: { protocol: 'hysteria', transport: 'hysteria', security: 'tls' }, inbounds: [{ tag: 'paid', protocol: 'hysteria', transport: 'hysteria', security: 'tls' }], fields: [{ key: 'hysteriaPorts', group: 'hysteria', kind: 'port-ranges', minimum: 1, maximum: 65535, unit: '', advanced: false, ownership: 'administrator' }], fieldErrors: [{ field: 'name', code: 'required' }], clients: [{ format: 'native', supported: true, reasons: [], omittedOptions: [] }] }
    const path = '/api/v1/admin/xnode/endpoints/preview'
    expect(isResponseCompatible(path, 'POST', response)).toBe(true)
    expect(isResponseCompatible(path, 'POST', { ...response, inbounds: undefined })).toBe(false)
    expect(isResponseCompatible(path, 'POST', { ...response, clients: [{ format: 'native', supported: 'yes', reasons: [], omittedOptions: [] }] })).toBe(false)
    expect(isResponseCompatible(path, 'POST', { ...response, credentials: { password: 'must-not-return' } })).toBe(false)
  })
  it('accepts fresh catalog contents without requiring a previous snapshot shape', () => {
    expect(isResponseCompatible('/api/v1/catalog', 'GET', {
      combos: [],
      addons: [],
      nodes: [],
      revision: 5,
    })).toBe(true)
  })

  it('rejects malformed cached catalog data before it reaches Vue state', () => {
    expect(isResponseCompatible('/api/v1/catalog', 'GET', {
      combos: null,
      addons: [],
      nodes: [],
    })).toBe(false)
  })

  it('does not claim a contract for an unknown route', () => {
    expect(isResponseCompatible('/api/v1/future-resource', 'GET', { any: 'shape' })).toBe(true)
  })

  it('accepts only the retired constant multiplier in current node response contracts', () => {
    const node = { id: 'node', name: 'Amsterdam', enabled: true, configId: 'config', multiplier: 1 }
    expect(isResponseCompatible('/api/v1/admin/xnode/nodes', 'GET', { items: [node] })).toBe(true)
    for (const multiplier of [0, 1.5, null, '1']) expect(isResponseCompatible('/api/v1/admin/xnode/nodes', 'GET', { items: [{ ...node, multiplier }] })).toBe(false)
    const catalog = { combos: [], addons: [], nodes: [{ uuid: 'node', name: 'Amsterdam', countryCode: 'NL', consumptionMultiplier: 1, providerName: null }] }
    expect(isResponseCompatible('/api/v1/catalog', 'GET', catalog)).toBe(true)
    expect(isResponseCompatible('/api/v1/catalog', 'GET', { ...catalog, nodes: [{ ...catalog.nodes[0], consumptionMultiplier: 2 }] })).toBe(false)
  })
})
