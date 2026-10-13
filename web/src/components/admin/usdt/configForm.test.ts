import { describe, expect, it } from 'vitest'
import type { USDTConfig } from '@/api/usdt'
import { configSchema, newEndpoint, newQuotaGroup } from './configForm'

function config(): USDTConfig {
  return { enabled: true, name: 'USDT', rate: '10', expiryMinutes: 30, groups: [newQuotaGroup('shared')], networks: [{ network: 'ethereum', enabled: true, addresses: ['0x1111111111111111111111111111111111111111'], endpoints: [{ ...newEndpoint('shared'), rpcUrl: 'wss://rpc.example/ws', useWs: true }] }] }
}

describe('USDT configuration contracts', () => {
  it('accepts EVM WS-only and preserves masked URLs and credentials', () => {
    const value = config()
    expect(configSchema().safeParse(value).success).toBe(true)
    value.networks[0]!.endpoints[0]!.rpcUrl = ''
    value.networks[0]!.endpoints[0]!.rpcConfigured = true
    value.networks[0]!.endpoints[0]!.wsConfigured = true
    expect(configSchema().safeParse(value).success).toBe(true)
  })
  it('rejects unsupported protocols, missing shared groups and unsafe rates', () => {
    for (const mutate of [
      (value: USDTConfig) => { value.rate = '1e4' },
      (value: USDTConfig) => { value.networks[0]!.endpoints[0]!.rpcUrl = 'file:///private' },
      (value: USDTConfig) => { value.networks[0]!.endpoints[0]!.group = 'missing' },
      (value: USDTConfig) => { value.networks[0]!.addresses = [] },
      (value: USDTConfig) => { value.groups[0]!.concurrency = 9 },
    ]) { const value = config(); mutate(value); expect(configSchema().safeParse(value).success).toBe(false) }
  })
  it('requires HTTP for Solana and an indexer for Aptos', () => {
    const value = config()
    value.networks[0]!.network = 'solana'
    expect(configSchema().safeParse(value).success).toBe(false)
    value.networks[0]!.network = 'aptos'
    value.networks[0]!.endpoints[0]!.rpcUrl = 'https://rpc.example'
    value.networks[0]!.endpoints[0]!.useWs = false
    expect(configSchema().safeParse(value).success).toBe(false)
    value.networks[0]!.endpoints[0]!.indexerUrl = 'https://indexer.example/graphql'
    expect(configSchema().safeParse(value).success).toBe(true)
  })
})
