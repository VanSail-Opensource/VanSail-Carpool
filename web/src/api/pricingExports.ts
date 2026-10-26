import { request, requestStream } from './http'

export interface PricingExportScope { from?: string; until?: string; users: string[]; nodes: string[]; products: string[]; category: string; format: 'csv' | 'jsonl'; includeUsers: boolean; includeFinancial: boolean; pseudonymize: boolean }
export const pricingExportsApi = {
  authorize: (scope: PricingExportScope, reason: string, signal: AbortSignal) => request<{ token: string; expiresAt: string }>('/api/v1/admin/pricing/exports/authorization', { method: 'POST', body: { scope, reason, authorize: true }, signal }),
  stream: (scope: PricingExportScope, authorizationToken: string, signal: AbortSignal) => requestStream('/api/v1/admin/pricing/exports', { method: 'POST', body: { scope, authorizationToken }, signal }),
}
