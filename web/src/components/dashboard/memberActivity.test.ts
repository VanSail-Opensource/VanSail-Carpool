import { describe, expect, it } from 'vitest'
import { memberActivity } from './memberActivity'

describe('native member activity', () => {
  const now = Date.parse('2026-10-19T00:00:00Z')
  const fresh = { activityStatus: 'online' as const, activityExpiresAt: new Date(now + 15000).toISOString() }
  it('expires native observations and clears failures instead of claiming offline', () => {
    expect(memberActivity(fresh, null, now)).toBe('online')
    expect(memberActivity(fresh, null, now + 15000)).toBe('unavailable')
    expect(memberActivity(fresh, 'unavailable', now)).toBe('unavailable')
    expect(memberActivity(null, null, now)).toBe('unavailable')
    expect(memberActivity({}, null, now)).toBe('unavailable')
    expect(memberActivity({ ...fresh, activityStatus: 'offline' }, null, now)).toBe('offline')
  })
})
