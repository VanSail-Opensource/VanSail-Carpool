import type { AdminEntitlement, AdminUserDetail } from '@/api/adminOperations'

export function hasPositiveBalance(minor: string): boolean {
  return /^\d+$/.test(minor) && BigInt(minor) > 0n
}

export function adminUserName(detail: AdminUserDetail): string {
  return detail.user.displayName || detail.user.telegramId
}

export function toLocalDateTime(value: string): string {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return ''
  const offset = parsed.getTimezoneOffset() * 60_000
  return new Date(parsed.getTime() - offset).toISOString().slice(0, 16)
}

export function fromLocalDateTime(value: string): string {
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? '' : parsed.toISOString()
}

export function entitlementTone(status: AdminEntitlement['status']): 'neutral' | 'success' | 'warning' | 'danger' {
  if (status === 'active') return 'success'
  if (status === 'activating' || status === 'queued') return 'warning'
  if (status === 'failed') return 'danger'
  return 'neutral'
}
