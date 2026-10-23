import { z } from 'zod'
import type { ConnectivityConfig } from '@/api/connectivity'

export const defaultConnectivityConfig: ConnectivityConfig = {
  scheduledEnabled: false, proxyUserId: 0, intervalSeconds: 300,
  timeoutSeconds: 15, maxRetries: 10, retryIntervalSeconds: 1, probeUrl: 'https://cp.cloudflare.com/generate_204',
}

const configSchema = z.object({
  scheduledEnabled: z.boolean(),
  proxyUserId: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  intervalSeconds: z.number().int().min(60).max(86400),
  timeoutSeconds: z.number().int().min(1).max(60),
  maxRetries: z.number().int().min(0).max(10),
  retryIntervalSeconds: z.number().int().min(1).max(60),
  probeUrl: z.string().max(2048).refine(value => {
    try {
      const url = new URL(value)
      return url.protocol === 'https:' && Boolean(url.hostname) && !url.username && !url.password
    } catch { return false }
  }),
}).refine(value => !value.scheduledEnabled || value.proxyUserId > 0)

export function validConnectivityConfig(config: ConnectivityConfig): boolean {
  return configSchema.safeParse(config).success
}

export function sameConnectivityConfig(a: ConnectivityConfig, b: ConnectivityConfig): boolean {
  return a.scheduledEnabled === b.scheduledEnabled && a.proxyUserId === b.proxyUserId
    && a.intervalSeconds === b.intervalSeconds && a.timeoutSeconds === b.timeoutSeconds
    && a.maxRetries === b.maxRetries && a.retryIntervalSeconds === b.retryIntervalSeconds && a.probeUrl === b.probeUrl
}
