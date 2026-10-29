import { pricingRead } from './pricingReads'
import { request, type QueryValue } from './http'
import type { PricingPolicy } from './pricing'

export interface PricingProduct { id: string; productType: 'combo' | 'addon' | 'payg'; productId: string; name: string; fixedPriceMinor: string; currentPriceMinor?: string | null; pricingStatus?: string; pricingReason?: string; calculatedAt?: string; currentPriceValidUntil?: string; policy: PricingPolicy }
export interface PricingHealth { nodeId: string; state: string; at: string; reportedAt: string; cpu: number; memory: number; since: string }
export interface PricingDashboard { products: PricingProduct[]; nextCursor: string; health: PricingHealth[]; storage: { pricingBytes: string; databaseBytes: string; pricingBudgetBytes: string; databaseTargetBytes: string; pressure: boolean }; datasets: string[]; summary?: PricingRecord }
export type PricingRecord = Record<string, string | number | boolean | null>
export interface PricingForecast { forecastId: string; entityType: string; entityId: string; metric: string; generatedAt: string; targetFrom: string; targetUntil: string; horizonSeconds: number; predictedP10: number; predictedP50: number; predictedP90: number; predictedMean: number; modelVersion: string; featureWatermark: string; featureSnapshotHash: string; dataCoverageBps: number; healthState: string; calibrationStatus: string; actual: number | null; status: string }
export interface PricingValidation { samples: number; excluded: number; mae: number; medianAbsoluteError: number; wape: number | null; bias: number; coverage: number; intervalWidth: number; pinballLoss: number }
export interface PricingIntervention { scopeType: string; scopeId: string; command: string; reason: string; priceMinor: string; discountBps: number; budgetMinor: string; priceVersion: string; expectedVersion: number; expiresAt: string }
export interface PricingCost { nodeId: string; cycleCostMinor: string; cycleDays: number; capacityBytes: string; effectiveAt: string; version: number; reason: string; actualPaidMinor?: string; chargeMode?: 'cycle' | 'hourly'; hourlyRateMicros?: string; trafficRateMicros?: string; capacityUnlimited?: boolean }
export const pricingAnalysisApi = {
  health: (signal?: AbortSignal) => request<{ health: PricingHealth[]; observedAt: string }>('/api/v1/admin/pricing/health', { signal }),
  dashboard: (cursor = '', signal?: AbortSignal) => request<PricingDashboard>('/api/v1/admin/pricing', { query: { cursor }, signal }),
  data: (dataset: string, query: Record<string, QueryValue> = {}, signal?: AbortSignal) => pricingRead(signal, () => request<{ items: PricingRecord[]; truncated: boolean; nextCursor?: string }>('/api/v1/admin/pricing/data', { query: { dataset, ...query }, signal })),
  history: (query: Record<string, QueryValue>, signal?: AbortSignal) => request<{ items: PricingForecast[]; metrics: PricingValidation }>('/api/v1/admin/pricing/history', { query, signal }),
  version: (scopeType: string, scopeId: string) => request<{ version: number }>('/api/v1/admin/pricing/interventions/version', { query: { scopeType, scopeId } }),
  intervene: (body: PricingIntervention) => request<{ id: string; status: string }>('/api/v1/admin/pricing/interventions', { method: 'POST', body }),
  cancel: (id: string, reason: string) => request<void>(`/api/v1/admin/pricing/interventions/${encodeURIComponent(id)}/cancel`, { method: 'POST', body: { reason } }),
  cost: (body: PricingCost) => request<void>(`/api/v1/admin/pricing/costs/${encodeURIComponent(body.nodeId)}`, { method: 'PUT', body }),
}
