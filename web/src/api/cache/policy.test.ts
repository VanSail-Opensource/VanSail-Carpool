import { describe, expect, it } from 'vitest'
import { isReadRequest, responseCacheKey } from './policy'

describe('live activity eligibility', () => {
  it('honors explicit no-store for otherwise cacheable resources', () => {
    for (const path of ['/api/v1/hourly', '/api/v1/admin/xnode/nodes/one/metrics', '/api/v1/admin/api-tokens']) {
      expect(responseCacheKey(path, { cache: 'no-store' })).toBeNull()
    }
    expect(responseCacheKey('/api/v1/dashboard')).not.toBeNull()
  })
  it('never persists member import keys', () => {
    expect(responseCacheKey('/api/v1/subscription')).toBeNull()
  })
  it('keeps exact financial breakdowns out of session snapshots', () => {
    expect(responseCacheKey('/api/v1/usage/breakdown', { cache: 'no-store' })).toBeNull()
    expect(responseCacheKey('/api/v1/usage/breakdown?refresh=1')).toBeNull()
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
