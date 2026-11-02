import type { XNodeUserMetrics } from '@/api/xnodeMember'

export type ActivityStatus = 'online' | 'offline' | 'unavailable'
export type ActivityMetrics = Pick<XNodeUserMetrics, 'activityStatus' | 'activityExpiresAt' | 'fetchedAt'>

// The server timestamps share a clock. Start the resulting TTL at request start
// so response latency can only shorten the period for which a sample is shown.
export function activityDeadline(value: ActivityMetrics, requestStartedAt: number): number {
  const expiry = Date.parse(value.activityExpiresAt ?? '')
  const fetched = Date.parse(value.fetchedAt)
  if (!Number.isFinite(expiry) || !Number.isFinite(fetched) || !Number.isFinite(requestStartedAt)) return NaN
  return requestStartedAt + (expiry - fetched)
}

export function memberActivity(value: ActivityMetrics | null, error: string | null, now: number, deadline: number): ActivityStatus {
  if (!value || error || !Number.isFinite(now) || !Number.isFinite(deadline) || deadline <= now) return 'unavailable'
  return value.activityStatus === 'online' || value.activityStatus === 'offline' ? value.activityStatus : 'unavailable'
}
