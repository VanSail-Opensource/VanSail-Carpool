import type {
  AdminResource,
  BackupRecord,
  BillingAmountLimits,
  Catalog,
  Dashboard,
  DashboardNodeUsage,
  LedgerEntry,
  OperationReceipt,
  Paginated,
  PaymentOperation,
  Purchase,
  PurchaseQuote,
  AutoRenewal,
  RolloverProjection,
  Session,
  SquadProduct,
  SquadProductWrite,
  StatisticsNodesSnapshot,
  StatisticsSnapshot,
} from './types'
import type { components } from './generated'
import type { FeaturePaymentMethod, FeaturePaymentOrder, FeaturePaymentReturnStatus, PaymentReturnProvider } from './features'
import { request, type QueryValue } from './http'
import { purchaseAddonsApi } from './purchaseAddons'
import { communityApi } from './community'
import { createUuid } from '@/utils/browserCompatibility'

export { ApiError } from './http'

type TelegramAuthRequest = components['schemas']['TelegramAuthRequest']
type PurchaseRequest = components['schemas']['PurchaseRequest']
type BalanceAdjustmentRequest = components['schemas']['BalanceAdjustmentRequest']
type ReasonRequest = components['schemas']['ReasonRequest']
type RefundRequest = components['schemas']['RefundRequest']
type AutoRenewalUpdate = components['schemas']['AutoRenewalUpdate']
type GeneratedSquadProductWrite = components['schemas']['SquadProductWrite']
export interface AdminPaymentProfile {
  id: string
  provider: 'ezpay' | 'bepusdt'
  providerName: string
  enabledChannels: string[]
  endpoint: string
  merchantId: string
  credential: string
  acknowledgement: string
  enabled: boolean
  configured: boolean
}
export type AdminPaymentProfileWrite = Omit<AdminPaymentProfile, 'configured'>

function paymentProfileWriteBody(profile: AdminPaymentProfileWrite): AdminPaymentProfileWrite {
  return {
    id: profile.id,
    provider: profile.provider,
    providerName: profile.providerName,
    enabledChannels: profile.enabledChannels,
    endpoint: profile.endpoint,
    merchantId: profile.merchantId,
    credential: profile.credential,
    acknowledgement: profile.acknowledgement,
    enabled: profile.enabled,
  }
}

const legacyJobRetryKeys = new Map<string, string>()

async function retryAdminJob(jobId: string, idempotencyKey?: string): Promise<OperationReceipt> {
  const key = idempotencyKey ?? legacyJobRetryKeys.get(jobId) ?? createUuid()
  if (!idempotencyKey) legacyJobRetryKeys.set(jobId, key)
  const receipt = await request<OperationReceipt>(`/api/v1/admin/jobs/${encodeURIComponent(jobId)}/retry`, {
    method: 'POST', headers: { 'Idempotency-Key': key },
  })
  if (!idempotencyKey) legacyJobRetryKeys.delete(jobId)
  return receipt
}

export const api = {
  ...purchaseAddonsApi,
  ...communityApi,
  authTelegram: (initData: string) => request<Session>('/api/v1/auth/telegram', {
    method: 'POST',
    body: { initData } satisfies TelegramAuthRequest,
  }),
  getMe: () => request<Session>('/api/v1/me'),
  verifyCaptcha: (token: string) => request<Session>('/api/v1/me/captcha', {
    method: 'POST', body: { token } satisfies components['schemas']['CaptchaVerifyRequest'],
  }),
  continueOnboarding: () => request<Session>('/api/v1/onboarding/continue', { method: 'POST' }),
  acceptAgreement: (revision: number, agreementIds: string[]) => request<Session>('/api/v1/onboarding/agreement', {
    method: 'POST',
    body: { revision, agreementIds },
  }),
  getDashboard: () => request<Dashboard>('/api/v1/dashboard'),
  getDashboardNodeUsage: (start: string, end: string) =>
    request<DashboardNodeUsage>('/api/v1/dashboard/node-usage', { query: { start, end } }),
  getStatistics: (signal?: AbortSignal) => request<StatisticsSnapshot>('/api/v1/admin/statistics', { cache: 'no-store', signal }),
  getStatisticsNodes: (signal?: AbortSignal) => request<StatisticsNodesSnapshot>('/api/v1/admin/statistics/nodes', { cache: 'no-store', signal }),
  getCatalog: () => request<Catalog>('/api/v1/catalog'),
  createPurchase: (comboId: string, squadProductIds: string[], couponGrantId: string | undefined, idempotencyKey: string, squadActivationCodes: Record<string, string> = {}) => request<Purchase>('/api/v1/purchases', {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: {
      comboId,
      addonSquadProductIds: squadProductIds,
      ...(couponGrantId ? { couponGrantId } : {}),
      ...(Object.keys(squadActivationCodes).length > 0 ? { squadActivationCodes } : {}),
    } as PurchaseRequest & { couponGrantId?: string },
  }),
  quotePurchase: (comboId: string, squadProductIds: string[], couponGrantId?: string) => request<PurchaseQuote>('/api/v1/purchases/quote', {
    method: 'POST',
    body: { comboId, addonSquadProductIds: squadProductIds, couponGrantId },
  }),
  getPurchases: () => request<Paginated<Purchase>>('/api/v1/purchases'),
  cancelQueuedPurchase: (purchaseId: string) => request<Purchase>(`/api/v1/purchases/${encodeURIComponent(purchaseId)}/cancel`, {
    method: 'POST',
  }),
  getPurchaseRollover: (purchaseId: string) => request<RolloverProjection>(`/api/v1/purchases/${encodeURIComponent(purchaseId)}/rollover`),
  getAutoRenewal: (purchaseId: string) => request<AutoRenewal>(`/api/v1/purchases/${encodeURIComponent(purchaseId)}/auto-renewal`),
  setAutoRenewal: (purchaseId: string, enabled: boolean) => request<AutoRenewal>(`/api/v1/purchases/${encodeURIComponent(purchaseId)}/auto-renewal`, {
    method: 'PUT', body: { enabled } satisfies AutoRenewalUpdate,
  }),
  revokeSubscription: (idempotencyKey: string) => request<OperationReceipt>('/api/v1/subscription/revoke', {
    method: 'POST', headers: { 'Idempotency-Key': idempotencyKey },
  }),
  getBalance: () => request<{ balance: Dashboard['balance']; paymentMethods: FeaturePaymentMethod[]; addAmountLimits: BillingAmountLimits; pendingPaymentOrder: FeaturePaymentOrder | null }>('/api/v1/balance'),
  createPaymentOrder: (methodId: string, txbMinorUnits: string, idempotencyKey: string) => request<PaymentOperation>('/api/v1/payments/orders', {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: { methodId, txbMinor: txbMinorUnits },
  }),
  getPaymentOrder: (id: string) => request<FeaturePaymentOrder>(`/api/v1/payments/orders/${encodeURIComponent(id)}`),
  getPaymentReturnStatus: (provider: PaymentReturnProvider, id: string, capability: string) => request<FeaturePaymentReturnStatus>(
    `/api/v1/payments/return/${provider}/${encodeURIComponent(id)}/status`, { query: { capability } },
  ),
  cancelPaymentOrder: (id: string, idempotencyKey: string) => request<PaymentOperation>(`/api/v1/payments/orders/${encodeURIComponent(id)}/cancel`, {
    method: 'POST', headers: { 'Idempotency-Key': idempotencyKey },
  }),
  getAdminResource: <T>(resource: AdminResource, query?: Record<string, QueryValue>) =>
    request<T>(`/api/v1/admin/${resource}`, { query }),
  updateAdminSetting: <T>(key: string, value: string) =>
    request<T>(`/api/v1/admin/settings/${encodeURIComponent(key)}`, { method: 'PUT', body: { value } }),
  importAdminSquadProducts: () =>
    request<{ items: SquadProduct[] }>('/api/v1/admin/squad-products/import', { method: 'POST' }),
  updateAdminSquadProduct: (id: string, body: SquadProductWrite) =>
    request<SquadProduct>(`/api/v1/admin/squad-products/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: body satisfies GeneratedSquadProductWrite,
    }),
  createAdminResource: <T>(resource: AdminResource, body: unknown) =>
    request<T>(`/api/v1/admin/${resource}`, { method: 'POST', body }),
  updateAdminResource: <T>(resource: AdminResource, id: string, body: unknown) =>
    request<T>(`/api/v1/admin/${resource}/${encodeURIComponent(id)}`, { method: 'PUT', body }),
  deleteAdminResource: (resource: AdminResource, id: string) =>
    request<void>(`/api/v1/admin/${resource}/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  refundPayment: (paymentId: string, reason: string, idempotencyKey: string) => request<OperationReceipt>(`/api/v1/admin/payments/${encodeURIComponent(paymentId)}/refund`, {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: { reason } satisfies RefundRequest,
  }),
  getAdminPaymentProfiles: () => request<{ items: AdminPaymentProfile[] }>('/api/v1/admin/payment-profiles'),
  createAdminPaymentProfile: (body: AdminPaymentProfileWrite) => request<AdminPaymentProfile>('/api/v1/admin/payment-profiles', { method: 'POST', body: paymentProfileWriteBody(body) }),
  updateAdminPaymentProfile: (id: string, body: AdminPaymentProfileWrite) => request<AdminPaymentProfile>(`/api/v1/admin/payment-profiles/${encodeURIComponent(id)}`, { method: 'PUT', body: paymentProfileWriteBody(body) }),
  deleteAdminPaymentProfile: (id: string) => request<void>(`/api/v1/admin/payment-profiles/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  cancelAdminEntitlement: (entitlementId: string, reason: string) => request<Purchase>(`/api/v1/admin/entitlements/${encodeURIComponent(entitlementId)}/cancel`, {
    method: 'POST',
    body: { reason } satisfies ReasonRequest,
  }),
  retryAdminJob,
  deleteAdminJob: (jobId: string) => request<void>(`/api/v1/admin/jobs/${encodeURIComponent(jobId)}`, { method: 'DELETE' }),
  runAdminMaintenance: (idempotencyKey: string) => request<OperationReceipt>('/api/v1/admin/maintenance', {
    method: 'POST', headers: { 'Idempotency-Key': idempotencyKey },
  }),
  createAdminBackup: () => request<BackupRecord>('/api/v1/admin/backups', { method: 'POST' }),
  adjustBalance: (userId: string, amountMinor: string, reason: string) => request<LedgerEntry>(`/api/v1/admin/users/${encodeURIComponent(userId)}/balance-adjustments`, {
    method: 'POST',
    body: { deltaTxbMinor: amountMinor, reason } satisfies BalanceAdjustmentRequest,
  }),
}
