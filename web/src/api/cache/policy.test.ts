import { describe, expect, it } from 'vitest'
import { isReadRequest, responseCacheKey } from './policy'

describe('live activity eligibility', () => {
  it('never persists member import keys or current uptime eligibility', () => {
    expect(responseCacheKey('/api/v1/subscription')).toBeNull()
    expect(responseCacheKey('/api/v1/connectivity/summary')).toBeNull()
  })
  it('never persists activity snapshots while retaining ordinary read caching', () => {
    expect(responseCacheKey('/api/v1/activity')).toBeNull()
    expect(responseCacheKey('/api/v1/activity?refresh=1')).toBeNull()
    expect(responseCacheKey('/api/v1/dashboard')).toBe('GET /api/v1/dashboard ')
  })
  it('does not cache or invalidate data for read-only firewall searches', () => {
    const path = '/api/v1/admin/xnode/firewall/records/search'
    expect(isReadRequest(path, 'POST')).toBe(true)
    expect(responseCacheKey(path, { method: 'POST', body: { filter: { op: 'and' }, limit: 50 } })).toBeNull()
  })
})
