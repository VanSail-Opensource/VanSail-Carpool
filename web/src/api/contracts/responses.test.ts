import { describe, expect, it } from 'vitest'
import { isResponseCompatible } from './responses'

describe('rendering snapshot compatibility', () => {
  it('validates endpoint draft preview facts while retaining additive response compatibility', () => {
    const response = { valid: false, metadata: { protocol: 'hysteria', transport: 'hysteria', security: 'tls' }, inbounds: [{ tag: 'paid', protocol: 'hysteria', transport: 'hysteria', security: 'tls' }], fields: [{ key: 'hysteriaPorts', group: 'hysteria', kind: 'port-ranges', minimum: 1, maximum: 65535, unit: '', advanced: false, ownership: 'administrator' }], fieldErrors: [{ field: 'name', code: 'required' }], clients: [{ format: 'native', supported: true, reasons: [], omittedOptions: [] }] }
    const path = '/api/v1/admin/xnode/endpoints/preview'
    expect(isResponseCompatible(path, 'POST', response)).toBe(true)
    expect(isResponseCompatible(path, 'POST', { ...response, inbounds: undefined })).toBe(false)
    expect(isResponseCompatible(path, 'POST', { ...response, clients: [{ format: 'native', supported: 'yes', reasons: [], omittedOptions: [] }] })).toBe(false)
    expect(isResponseCompatible(path, 'POST', { ...response, futureOptionalFact: 'compatible' })).toBe(true)
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

  it('accepts issued ledger categories and retained financial evidence', () => {
    const money = { currency: 'TXB', minor: '100', display: '1.00' }
    const entry = { id: 'ledger', delta: money, balanceAfter: money, referenceId: 'operation', note: '', createdAt: '2026-11-13T00:00:00Z' }
    const kinds = [
      'purchase_addon_debit', 'telegram_deduct', 'admin_entitlement_refund', 'member_refund_credit',
      'traffic_reset_debit', 'traffic_reset_compensation', 'activity_group_message_reward',
      'activity_raffle_fee', 'activity_raffle_reserve', 'activity_raffle_reserve_adjust',
      'activity_raffle_reserve_release', 'activity_raffle_refund', 'affiliate_commission',
      'affiliate_tier_reward', 'ip_lookup', 'ip_lookup_refresh', 'ip_lookup_refund',
      'questionnaire_reward',
    ]
    for (const kind of kinds) {
      expect(isResponseCompatible('/api/v1/ledger', 'GET', { items: [{ ...entry, kind }] }), kind).toBe(true)
    }
    expect(isResponseCompatible('/api/v1/ledger', 'GET', { items: [{ ...entry, kind: 'not_a_ledger_kind' }] })).toBe(false)
  })

  it('accepts current node responses without retired multiplier metadata', () => {
    const node = { id: 'node', name: 'Amsterdam', enabled: true, configId: 'config' }
    expect(isResponseCompatible('/api/v1/admin/xnode/nodes', 'GET', { items: [node] })).toBe(true)
    expect(isResponseCompatible('/api/v1/admin/xnode/nodes', 'GET', { items: [{ ...node, enabled: 'true' }] })).toBe(false)
    const catalog = { combos: [], addons: [], nodes: [{ uuid: 'node', name: 'Amsterdam', countryCode: 'NL', consumptionMultiplier: 1, providerName: null }] }
    expect(isResponseCompatible('/api/v1/catalog', 'GET', catalog)).toBe(true)
    expect(isResponseCompatible('/api/v1/catalog', 'GET', { ...catalog, nodes: [{ ...catalog.nodes[0], consumptionMultiplier: 2 }] })).toBe(false)
  })
})
