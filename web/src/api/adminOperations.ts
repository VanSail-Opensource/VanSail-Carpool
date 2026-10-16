import type { components } from './generated'
import { request, streamAdminBackupUpload } from './http'

export type AdminUserDetail = components['schemas']['AdminUserDetail']
export type AdminEntitlement = components['schemas']['AdminEntitlement']
export type OperationReceipt = components['schemas']['OperationReceipt']
export type EntitlementEditRequest = components['schemas']['EntitlementEditRequest']
export type AdminEntitlementRefundRequest = components['schemas']['AdminEntitlementRefundRequest']
export type AdminEntitlementRefundQuote = components['schemas']['AdminEntitlementRefundQuote']
export type ComboReplacementRequest = components['schemas']['ComboReplacementRequest']
export type BulkExtensionRequest = components['schemas']['BulkExtensionRequest']
export type BulkExtensionPreview = components['schemas']['BulkExtensionPreview']
export type BackupRun = components['schemas']['BackupRun']
export type Combo = components['schemas']['Combo']
export type SquadProduct = components['schemas']['SquadProduct']
export type CourtesyCredit = components['schemas']['CourtesyCredit']
export type OperationResolution = components['schemas']['OperationResolutionRequest']['resolution']

export interface AdminCatalogOptions {
  combos: Combo[]
  squads: SquadProduct[]
}

const keyHeader = (key: string) => ({ 'Idempotency-Key': key })

export const adminOperationsApi = {
  getUser: (userId: string) => request<AdminUserDetail>(`/api/v1/admin/users/${encodeURIComponent(userId)}`),
  getEntitlementRefundQuote: (userId: string, entitlementId: string) => request<AdminEntitlementRefundQuote>(`/api/v1/admin/users/${encodeURIComponent(userId)}/entitlements/${encodeURIComponent(entitlementId)}/refund`, { cache: 'no-store' }),
  unblockIP: (userId: string, blockId: string, key: string) =>
    request<OperationReceipt>(`/api/v1/admin/users/${encodeURIComponent(userId)}/ip-blocks/${encodeURIComponent(blockId)}/unblock`, {
      method: 'POST', headers: keyHeader(key),
    }),
  getCatalogOptions: async (): Promise<AdminCatalogOptions> => {
    const [combos, squads] = await Promise.all([
      request<{ items: Combo[] }>('/api/v1/admin/combos'),
      request<{ items: SquadProduct[] }>('/api/v1/admin/squad-products'),
    ])
    return { combos: combos.items, squads: squads.items }
  },
  editEntitlement: (userId: string, entitlementId: string, body: EntitlementEditRequest, key: string) =>
    request<AdminEntitlement>(`/api/v1/admin/users/${encodeURIComponent(userId)}/entitlements/${encodeURIComponent(entitlementId)}`, {
      method: 'PUT', headers: keyHeader(key), body,
    }),
	refundEntitlement: (userId: string, entitlementId: string, body: AdminEntitlementRefundRequest, key: string) =>
		request<OperationReceipt>(`/api/v1/admin/users/${encodeURIComponent(userId)}/entitlements/${encodeURIComponent(entitlementId)}/refund`, {
			method: 'POST', headers: keyHeader(key), body,
		}),
	grantUserCoupon: (userId: string, couponId: string, reason: string, key: string) =>
		request<components['schemas']['CouponGrant']>(`/api/v1/admin/users/${encodeURIComponent(userId)}/coupon-grants`, {
			method: 'POST', headers: keyHeader(key), body: { couponId, reason },
		}),
	discardUserCoupon: (userId: string, grantId: string, key: string) =>
		request<void>(`/api/v1/admin/users/${encodeURIComponent(userId)}/coupon-grants/${encodeURIComponent(grantId)}`, {
			method: 'DELETE', headers: keyHeader(key),
		}),
	temporaryBan: (userId: string, durationMinutes: number, reason: string, key: string) =>
		request<OperationReceipt>(`/api/v1/admin/users/${encodeURIComponent(userId)}/temporary-ban`, {
			method: 'POST', headers: keyHeader(key), body: { durationMinutes, reason },
		}),
	temporaryUnban: (userId: string, reason: string, key: string) =>
		request<OperationReceipt>(`/api/v1/admin/users/${encodeURIComponent(userId)}/temporary-ban/unban`, {
			method: 'POST', headers: keyHeader(key), body: { reason },
		}),

	requestUserConnections: (userId: string, key: string) =>
		request<components['schemas']['ConnectionScan']>(`/api/v1/admin/users/${encodeURIComponent(userId)}/connections`, {
			method: 'POST', headers: keyHeader(key),
		}),
	pollUserConnections: (userId: string, scanId: string) =>
		request<components['schemas']['ConnectionScan']>(`/api/v1/admin/users/${encodeURIComponent(userId)}/connections/${encodeURIComponent(scanId)}`),
  refundPayment: (paymentId: string, reason: string, key: string) =>
    request<OperationReceipt>(`/api/v1/admin/payments/${encodeURIComponent(paymentId)}/refund`, {
      method: 'POST', headers: keyHeader(key), body: { reason },
    }),
  creditPayment: (paymentId: string, reason: string) =>
    request<CourtesyCredit>(`/api/v1/admin/payments/${encodeURIComponent(paymentId)}/courtesy-credit`, {
      method: 'POST', body: { reason },
    }),
  replaceCombo: (userId: string, body: ComboReplacementRequest, key: string) =>
    request<OperationReceipt>(`/api/v1/admin/users/${encodeURIComponent(userId)}/combo-replacement`, {
      method: 'POST', headers: keyHeader(key), body,
    }),
  previewBulkExtension: (body: BulkExtensionRequest) =>
    request<BulkExtensionPreview>('/api/v1/admin/bulk-extensions/preview', { method: 'POST', body }),
  createBulkExtension: (body: BulkExtensionRequest & { reason: string }, key: string) =>
    request<OperationReceipt>('/api/v1/admin/bulk-extensions', { method: 'POST', headers: keyHeader(key), body }),
  resolveOperation: (operationId: string, resolution: OperationResolution, reason: string, key: string) =>
    request<OperationReceipt>(`/api/v1/admin/operations/${encodeURIComponent(operationId)}/resolve`, {
      method: 'POST', headers: keyHeader(key), body: { resolution, reason },
    }),
  uploadBackup: (file: File, sha256: string, key: string) => {
    const form = new FormData()
    form.append('file', file, file.name)
    if (sha256) form.append('sha256', sha256)
    return streamAdminBackupUpload<BackupRun>(form, key)
  },
}
