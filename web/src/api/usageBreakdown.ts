import type { components } from './generated'
import type { DeepReadonly } from './types'
import { ApiError, request } from './http'
import { isResponseCompatible } from './contracts/responses'
import { hasValidUsageUnits } from './usageBreakdownValidation'

export type UsageBreakdown = DeepReadonly<components['schemas']['UsageBreakdown']>
export type RationalMinor = DeepReadonly<components['schemas']['RationalMinor']>

export const usageBreakdownApi = {
  get: async (signal?: AbortSignal): Promise<UsageBreakdown> => {
    const path = '/api/v1/usage/breakdown'
    const value = await request<UsageBreakdown>(path, { signal, cache: 'no-store' })
    if (!isResponseCompatible(path, 'GET', value) || !hasValidUsageUnits(value)) {
      throw new ApiError(502, { code: 'API_RESPONSE_INCOMPATIBLE', message: 'API_RESPONSE_INCOMPATIBLE' })
    }
    return value
  },
}
