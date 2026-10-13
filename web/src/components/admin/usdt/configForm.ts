import * as z from 'zod'
import type { USDTConfig, USDTEndpoint, USDTNetworkID, USDTQuotaGroup } from '@/api/usdt'
import { t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

export const networkRails: Record<USDTNetworkID, string> = {
  tron: 'usdt.trc20', ethereum: 'usdt.erc20', bsc: 'usdt.bep20', polygon: 'usdt.polygon',
  arbitrum: 'usdt.arbitrum', xlayer: 'usdt.xlayer', plasma: 'usdt.plasma',
  solana: 'usdt.solana', aptos: 'usdt.aptos', ton: 'usdt.ton',
}
export const evmNetworks = new Set<USDTNetworkID>(['ethereum', 'bsc', 'polygon', 'arbitrum', 'xlayer', 'plasma'])
export const supportsWS = (network: USDTNetworkID) => evmNetworks.has(network) || network === 'solana'

export function newQuotaGroup(id = createUuid()): USDTQuotaGroup {
  return { id, requestsPerSecond: 1, requestsPerMinute: 60, requestsPerDay: 20000, concurrency: 1 }
}
export function newEndpoint(group: string): USDTEndpoint {
  return { id: createUuid(), group, rpcUrl: '', wsUrl: '', indexerUrl: '', apiKey: '', useWs: false,
    rpcConfigured: false, wsConfigured: false, indexerConfigured: false, keyConfigured: false, host: '' }
}

export function configSchema() {
  const invalid = t('adminUsdt.invalid')
  const id = z.string().regex(/^[a-zA-Z0-9_-]{1,64}$/, invalid)
  const group = z.object({ id, requestsPerSecond: z.number().int().min(1).max(50), requestsPerMinute: z.number().int().min(1).max(3000), requestsPerDay: z.number().int().min(1).max(1000000), concurrency: z.number().int().min(1).max(8) })
  const endpoint = z.object({ id, group: id, rpcUrl: z.string(), wsUrl: z.string(), indexerUrl: z.string(), apiKey: z.string().max(4096), useWs: z.boolean(), rpcConfigured: z.boolean(), wsConfigured: z.boolean(), indexerConfigured: z.boolean(), keyConfigured: z.boolean(), host: z.string() })
  return z.object({ enabled: z.boolean(), name: z.string().trim().min(1, invalid).max(64),
    rate: z.string().refine(value => value === '' || (/^\d{1,18}(\.\d{1,12})?$/.test(value) && Number(value) > 0), invalid),
    expiryMinutes: z.number().int().min(5).max(120), groups: z.array(group).max(32),
    networks: z.array(z.object({ network: z.enum(Object.keys(networkRails) as [USDTNetworkID, ...USDTNetworkID[]]), enabled: z.boolean(), addresses: z.array(z.string()).max(128), endpoints: z.array(endpoint).max(8) })).max(10),
  }).superRefine((value, ctx) => {
    if (value.enabled && !value.rate) ctx.addIssue({ code: 'custom', path: ['rate'], message: invalid })
    const groups = new Set(value.groups.map(item => item.id))
    if (groups.size !== value.groups.length) ctx.addIssue({ code: 'custom', path: ['groups'], message: invalid })
    value.networks.forEach((network, index) => {
      if (network.enabled && (!network.addresses.length || !network.endpoints.length)) ctx.addIssue({ code: 'custom', path: ['networks', index], message: invalid })
      network.endpoints.forEach((item, endpointIndex) => {
        const path = ['networks', index, 'endpoints', endpointIndex]
        if (!groups.has(item.group)) ctx.addIssue({ code: 'custom', path: [...path, 'group'], message: invalid })
        for (const field of ['rpcUrl', 'wsUrl', 'indexerUrl'] as const) {
          if (!item[field]) continue
          try {
            const parsed = new URL(item[field])
            const schemes = field === 'wsUrl' ? ['ws:', 'wss:'] : field === 'rpcUrl' && evmNetworks.has(network.network) ? ['http:', 'https:', 'ws:', 'wss:'] : ['http:', 'https:']
            if (!schemes.includes(parsed.protocol) || !parsed.hostname || parsed.username || parsed.password || parsed.hash) throw new Error()
          } catch { ctx.addIssue({ code: 'custom', path: [...path, field], message: invalid }) }
        }
        if (!item.rpcUrl && !item.rpcConfigured) ctx.addIssue({ code: 'custom', path: [...path, 'rpcUrl'], message: invalid })
        if (network.network === 'aptos' && !item.indexerUrl && !item.indexerConfigured) ctx.addIssue({ code: 'custom', path: [...path, 'indexerUrl'], message: invalid })
        if (item.useWs && !item.wsUrl && !item.wsConfigured && !item.rpcUrl.startsWith('ws')) ctx.addIssue({ code: 'custom', path: [...path, 'wsUrl'], message: invalid })
      })
    })
  }) satisfies z.ZodType<USDTConfig>
}
