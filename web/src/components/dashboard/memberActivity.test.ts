import { describe, expect, it, vi } from 'vitest'
import { activityDeadline, memberActivity } from './memberActivity'

describe('native member activity', () => {
  const now = Date.parse('2026-10-19T00:00:00Z')
  const fresh = { activityStatus: 'online' as const, fetchedAt: new Date(now).toISOString(), activityExpiresAt: new Date(now + 15000).toISOString() }
  it('expires native observations and clears failures instead of claiming offline', () => {
    const deadline = activityDeadline(fresh, 1000)
    expect(memberActivity(fresh, null, 1000, deadline)).toBe('online')
    expect(memberActivity(fresh, null, 16000, deadline)).toBe('unavailable')
    expect(memberActivity(fresh, 'unavailable', 1000, deadline)).toBe('unavailable')
    expect(memberActivity(null, null, 1000, deadline)).toBe('unavailable')
    expect(memberActivity({ ...fresh, activityStatus: 'offline' }, null, 1000, deadline)).toBe('offline')
  })
  it('uses server-relative lifetime and counts the whole request against freshness', () => {
    const deadline = activityDeadline(fresh, 1000)
    expect(deadline).toBe(16000)
    const localClock = vi.spyOn(Date, 'now').mockReturnValue(now + 86400_000)
    expect(memberActivity(fresh, null, 1000, deadline)).toBe('online')
    localClock.mockReturnValue(now - 86400_000)
    expect(memberActivity(fresh, null, 16001, deadline)).toBe('unavailable')
    localClock.mockRestore()
    expect(memberActivity(fresh, null, 15000, deadline)).toBe('online')
    expect(memberActivity(fresh, null, 16001, deadline)).toBe('unavailable')
    expect(memberActivity(fresh, null, 17000, deadline)).toBe('unavailable')
  })
  it('rejects missing, invalid, or already expired server times', () => {
    for (const value of [
      { ...fresh, fetchedAt: 'invalid' },
      { ...fresh, activityExpiresAt: null },
      { ...fresh, activityExpiresAt: new Date(now - 1).toISOString() },
    ]) {
      expect(memberActivity(value, null, 1000, activityDeadline(value, 1000))).toBe('unavailable')
    }
  })
})
