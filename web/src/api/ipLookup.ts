import type { components } from './generated'
import { request } from './http'
import { streamIPDetails, type LiveEvent } from './ipLookupStream'

export type IPLookupProxySourceTest = components['schemas']['IPLookupProxySourceTest']
export type IPLookupProxySourceCacheStatus = components['schemas']['IPLookupProxySourceCacheStatus']
export type IPLookupProxySourcesStatus = components['schemas']['IPLookupProxySourcesStatus']
export type IPLookupState = components['schemas']['IPLookupState']
export type IPLookupQuote = components['schemas']['IPLookupQuoteResponse']
type IPLookupSubmission = components['schemas']['IPLookupQuote']
export type IPLookupCheck = components['schemas']['IPLookupCheck']
export type IPLookupReport = components['schemas']['IPLookupReport']
export type IPLookupAdminSettings = components['schemas']['IPLookupAdminSettings']
export type IPLookupRetryPolicy = components['schemas']['IPLookupRetryPolicy']
export type IPLookupLiveDetails = components['schemas']['IPLookupLiveDetails']
export type IPLookupLiveMeta = components['schemas']['IPLookupLiveMeta']
export type IPLookupBGPTopology = components['schemas']['IPLookupBGPTopology']
export type IPLookupASNDetails = components['schemas']['IPLookupASNDetails']
export type IPLookupTrafficSummary = components['schemas']['IPLookupTrafficSummary']
export type IPLookupProgress = components['schemas']['IPLookupProgress']
export type IPLookupProgressTask = components['schemas']['IPLookupProgressTask']
export type IPLookupSharedProgress = components['schemas']['IPLookupSharedProgress']
export type IPLookupLiveEvent = LiveEvent

export const ipLookupApi = {
  sourcesStatus: (signal?: AbortSignal) => request<IPLookupProxySourcesStatus>('/api/v1/admin/ip-lookup/proxy-sources', { cache: 'no-store', signal }),
  testSource: (id: string) => request<IPLookupProxySourceTest>(`/api/v1/admin/ip-lookup/proxy-sources/${encodeURIComponent(id)}/test`, { method: 'POST', cache: 'no-store' }),
  sourceTest: (id: string, signal?: AbortSignal) => request<IPLookupProxySourceTest>(`/api/v1/admin/ip-lookup/proxy-source-tests/${encodeURIComponent(id)}`, { cache: 'no-store', signal }),
  progress: (ip: string, signal?: AbortSignal) => request<IPLookupSharedProgress | null>('/api/v1/ip-lookup/progress', { query: { ip }, cache: 'no-store', signal }),
  details: (ip: string, signal?: AbortSignal, refresh = false, onEvent?: (event: LiveEvent) => void) => onEvent
    ? streamIPDetails(ip, signal, refresh, onEvent)
    : request<IPLookupLiveDetails>('/api/v1/ip-lookup/details', { query: { ip, ...(refresh ? { refresh } : {}) }, cache: 'no-store', signal }),
  state: () => request<IPLookupState>('/api/v1/ip-lookup', { cache: 'no-store' }),
  quote: (ip: string, refresh: boolean) => request<IPLookupQuote>('/api/v1/ip-lookup/quote', { method: 'POST', body: { ip, refresh }, cache: 'no-store' }),
  submit: (quote: IPLookupQuote, key: string) => {
    const body: IPLookupSubmission = {
      ip: quote.ip, refresh: quote.refresh, charge: quote.charge, useQuota: quote.useQuota,
      purchaseId: quote.purchaseId, remaining: quote.remaining, cacheReportId: quote.cacheReportId,
      configHash: quote.configHash, expiresAt: quote.expiresAt, token: quote.token,
      ...(quote.quotaCost ? { quotaCost: quote.quotaCost } : {}),
    }
    return request<components['schemas']['OperationReceipt']>('/api/v1/ip-lookup/checks', { method: 'POST', body, headers: { 'Idempotency-Key': key } })
  },
  check: (id: string) => request<IPLookupCheck>(`/api/v1/ip-lookup/checks/${encodeURIComponent(id)}`, { cache: 'no-store' }),
  settings: () => request<IPLookupAdminSettings>('/api/v1/admin/ip-lookup', { cache: 'no-store' }),
  saveSettings: (value: IPLookupAdminSettings) => request<IPLookupAdminSettings>('/api/v1/admin/ip-lookup', { method: 'PUT', body: value }),
}
