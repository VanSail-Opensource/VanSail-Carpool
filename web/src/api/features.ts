import { request, requestBlob } from './http'
import type { OperationReceipt } from './types'
import type { ActivityOverview, ActivityResult, ActivitySettings, ActivitySettingsWrite, BetGame, LuckyDrawAdmin, LuckyDrawWrite, LuckyDrawForecast } from './contracts/activity'
import type { CouponDefinition, CouponGrant, CouponRedemption } from './contracts/community'
import type { EmbyAccount, EmbyOverview } from './contracts/commerce'
import type { AdminStatistics, DatabaseMutationInput, DatabaseMutationResult, DatabaseMutationReview, DatabaseQueryInput, DatabaseRowsPage, DatabaseTable, OnboardingBundle, OnboardingLocalizedContent, PublishedOnboarding, RestoreOperation, StatisticsQuery } from './contracts/admin'
import type { AdminAffiliateView, AffiliateOverview, AffiliateReferralPage, AffiliateTier } from './contracts/affiliates'

export type * from './contracts/activity'
export type * from './contracts/community'
export type * from './contracts/commerce'
export type * from './contracts/admin'
export type * from './contracts/affiliates'

export type PaymentReturnProvider = 'ezpay' | 'bepusdt'

export interface FeaturePaymentReturnStatus {
  status: import('./types').PaymentStatus
  id: string
  provider: import('./types').PaymentProvider
  providerRail: string
  txb: import('./types').Money
  payableAmount: string
  payableCurrency: 'CNY' | 'USD' | 'USDT' | 'XTR'
  actualCryptoAmount: string | null
  actualCryptoCurrency: 'USDT' | 'USDC' | null
  createdAt: import('./types').RFC3339
  paidAt: import('./types').RFC3339 | null
}


const featureRequest = request

export const featuresApi = {
  getAffiliates: () => featureRequest<AffiliateOverview>('/api/v1/affiliates'),
  getAffiliateReferrals: (page: number) => featureRequest<AffiliateReferralPage>('/api/v1/affiliates/referrals', { query: { page } }),
  getAdminAffiliates: () => featureRequest<AdminAffiliateView>('/api/v1/admin/affiliates'),
  saveAdminAffiliates: (expectedVersion: number, tiers: AffiliateTier[]) => featureRequest<AdminAffiliateView>('/api/v1/admin/affiliates', {
    method: 'PUT', body: { expectedVersion, tiers },
  }),
  getActivity: () => featureRequest<ActivityOverview>('/api/v1/activity'),
  checkIn: (idempotencyKey: string) => featureRequest<ActivityResult>('/api/v1/activity/check-ins', { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey } }),
  placeBet: (gameId: string, stakeTxbMinor: string, idempotencyKey: string) => featureRequest<ActivityResult>('/api/v1/activity/bets', {
    method: 'POST', body: { gameId, stakeTxbMinor }, headers: { 'Idempotency-Key': idempotencyKey },
  }),
  drawLuckyPrize: (drawId: string, idempotencyKey: string) => featureRequest<ActivityResult>('/api/v1/activity/draws', {
    method: 'POST', body: { drawId }, headers: { 'Idempotency-Key': idempotencyKey },
  }),
  getCouponWallet: () => featureRequest<{ items: CouponGrant[] }>('/api/v1/coupons/wallet'),
  discardCouponWalletGrant: (id: string) => featureRequest<void>(`/api/v1/coupons/wallet/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  redeemCoupon: (code: string, idempotencyKey: string) => featureRequest<CouponRedemption>('/api/v1/coupons/redeem', {
    method: 'POST', body: { code }, headers: { 'Idempotency-Key': idempotencyKey },
  }),
  getEmby: () => featureRequest<EmbyOverview>('/api/v1/emby/account'),
  setupEmby: (body: { password: string; maxParentalRating: number | null; disabledLibraryIds: string[] }, idempotencyKey: string) =>
    featureRequest<OperationReceipt>('/api/v1/emby/setup', { method: 'POST', body, headers: { 'Idempotency-Key': idempotencyKey } }),
  updateEmbyPreferences: (body: { maxParentalRating: number | null; disabledLibraryIds: string[] }, idempotencyKey: string) =>
    featureRequest<OperationReceipt>('/api/v1/emby/preferences', { method: 'PUT', body, headers: { 'Idempotency-Key': idempotencyKey } }),
  changeEmbyPassword: (password: string, idempotencyKey: string) => featureRequest<OperationReceipt>('/api/v1/emby/password', {
    method: 'PUT', body: { password }, headers: { 'Idempotency-Key': idempotencyKey },
  }),
  getAdminActivityGames: () => featureRequest<{ items: BetGame[] }>('/api/v1/admin/activity-games'),
  getAdminActivitySettings: () => featureRequest<ActivitySettings>('/api/v1/admin/activity-settings'),
  saveAdminActivitySettings: (body: ActivitySettingsWrite) =>
    featureRequest<ActivitySettings>('/api/v1/admin/activity-settings', { method: 'PUT', body }),
  saveAdminActivityGame: (id: string | null, body: Omit<BetGame, 'id'>) => featureRequest<BetGame>(
    id ? `/api/v1/admin/activity-games/${encodeURIComponent(id)}` : '/api/v1/admin/activity-games',
    { method: id ? 'PUT' : 'POST', body },
  ),
  deleteAdminActivityGame: (id: string) => featureRequest<void>(`/api/v1/admin/activity-games/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  getAdminActivityGameStatistics: (id: string, query: StatisticsQuery = {}) =>
    featureRequest<AdminStatistics>(`/api/v1/admin/activity-games/${encodeURIComponent(id)}/statistics`, { query: { from: query.from, to: query.to, bucket: query.bucket, timeZone: query.timeZone } }),
  getAdminLuckyDraws: () => featureRequest<{ items: LuckyDrawAdmin[] }>('/api/v1/admin/lucky-draw'),
  saveAdminLuckyDraw: (id: string | null, body: LuckyDrawWrite) => featureRequest<LuckyDrawAdmin>(
    id ? `/api/v1/admin/lucky-draw/${encodeURIComponent(id)}` : '/api/v1/admin/lucky-draw',
    { method: id ? 'PUT' : 'POST', body },
  ),
  deleteAdminLuckyDraw: (id: string) => featureRequest<void>(`/api/v1/admin/lucky-draw/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  publishAdminLuckyDraw: (id: string) => featureRequest<void>(`/api/v1/admin/lucky-draw/${encodeURIComponent(id)}/publish`, { method: 'POST' }),
  getAdminLuckyDrawForecast: (id: string) => featureRequest<LuckyDrawForecast>(`/api/v1/admin/lucky-draw/${encodeURIComponent(id)}/forecast`),
  getAdminLuckyDrawStatistics: (id: string, query: StatisticsQuery = {}) =>
    featureRequest<AdminStatistics>(`/api/v1/admin/lucky-draw/${encodeURIComponent(id)}/statistics`, { query: { from: query.from, to: query.to, bucket: query.bucket, timeZone: query.timeZone } }),
  getAdminCoupons: () => featureRequest<{ items: CouponDefinition[] }>('/api/v1/admin/coupons'),
  getAdminComboStatistics: (id: string, query: StatisticsQuery = {}) =>
    featureRequest<AdminStatistics>(`/api/v1/admin/combos/${encodeURIComponent(id)}/statistics`, { query: { from: query.from, to: query.to, bucket: query.bucket, timeZone: query.timeZone } }),
  getAdminSquadStatistics: (id: string, query: StatisticsQuery = {}) =>
    featureRequest<AdminStatistics>(`/api/v1/admin/squad-products/${encodeURIComponent(id)}/statistics`, { query: { from: query.from, to: query.to, bucket: query.bucket, timeZone: query.timeZone } }),
  saveAdminCoupon: (id: string | null, body: Partial<CouponDefinition>) => featureRequest<CouponDefinition>(
    id ? `/api/v1/admin/coupons/${encodeURIComponent(id)}` : '/api/v1/admin/coupons',
    { method: id ? 'PUT' : 'POST', body },
  ),
  deactivateAdminCoupon: (id: string) => featureRequest<void>(`/api/v1/admin/coupons/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  getAdminEmbyAccounts: () => featureRequest<{ items: EmbyAccount[] }>('/api/v1/admin/emby-accounts'),
  retryAdminEmbyAccount: (id: string, idempotencyKey: string) => featureRequest<OperationReceipt>(`/api/v1/admin/emby-accounts/${encodeURIComponent(id)}/retry`, {
    method: 'POST', headers: { 'Idempotency-Key': idempotencyKey },
  }),
  downloadBackup: async (id: string) => {
    return requestBlob(`/api/v1/admin/backups/${encodeURIComponent(id)}/download`)
  },
  restoreBackup: (id: string, reason: string, confirmation: string, idempotencyKey: string) => featureRequest<RestoreOperation>(`/api/v1/admin/backups/${encodeURIComponent(id)}/restore`, {
    method: 'POST', body: { reason, confirmation }, headers: { 'Idempotency-Key': idempotencyKey },
  }),
  deleteBackup: (id: string) => featureRequest<void>(`/api/v1/admin/backups/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  getRestoreStatus: (id: string) => featureRequest<RestoreOperation>(`/api/v1/admin/restores/${encodeURIComponent(id)}`),
  getDatabaseTables: () => featureRequest<{ items: DatabaseTable[] }>('/api/v1/admin/database/tables'),
  getDatabaseRows: (table: string, cursor?: string) => featureRequest<DatabaseRowsPage>(
    `/api/v1/admin/database/tables/${encodeURIComponent(table)}/rows`, { query: { cursor, limit: 50 } },
  ),
  queryDatabaseRows: (table: string, body: DatabaseQueryInput) => featureRequest<DatabaseRowsPage>(
    `/api/v1/admin/database/tables/${encodeURIComponent(table)}/query`, { method: 'POST', body },
  ),
  getPublishedOnboarding: (locale: string) => featureRequest<PublishedOnboarding>('/api/v1/onboarding/content', { query: { locale } }),
  getAdminOnboardingBundle: (kind: 'welcome' | 'agreements') => featureRequest<OnboardingBundle>(`/api/v1/admin/onboarding/content/${kind}`),
  saveAdminOnboardingDraft: (kind: 'welcome' | 'agreements', draftRevision: number, content: OnboardingLocalizedContent) =>
    featureRequest<OnboardingBundle>(`/api/v1/admin/onboarding/content/${kind}/draft`, { method: 'PUT', body: { draftRevision, content } }),
  publishAdminOnboarding: (kind: 'welcome' | 'agreements', draftRevision: number) =>
    featureRequest<OnboardingBundle>(`/api/v1/admin/onboarding/content/${kind}/publish`, { method: 'POST', body: { draftRevision } }),
  reviewDatabaseMutation: (body: DatabaseMutationInput) => featureRequest<DatabaseMutationReview>('/api/v1/admin/database/mutations/review', {
    method: 'POST', body,
  }),
  applyDatabaseMutation: (body: DatabaseMutationInput & { reviewHash: string; confirmation: string }) =>
    featureRequest<DatabaseMutationResult>('/api/v1/admin/database/mutations', { method: 'POST', body }),
}
