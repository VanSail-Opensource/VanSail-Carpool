import type { XNodeUserMetrics } from '@/api/xnodeMember'

export type ActivityStatus = 'online' | 'offline' | 'unavailable'

export function memberActivity(value: Pick<XNodeUserMetrics, 'activityStatus' | 'activityExpiresAt'> | null, error: string | null, now: number): ActivityStatus {
  if (!value || error || !value.activityExpiresAt) return 'unavailable'
  const expiry = Date.parse(value.activityExpiresAt)
  if (!Number.isFinite(expiry) || expiry <= now) return 'unavailable'
  return value.activityStatus === 'online' || value.activityStatus === 'offline' ? value.activityStatus : 'unavailable'
}
