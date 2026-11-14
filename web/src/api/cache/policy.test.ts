import { describe, expect, it } from 'vitest'
import { isReadRequest, responseCacheKey } from './policy'

describe('display cache and live capability boundaries', () => {
  it('honors explicit no-store for otherwise cacheable resources', () => {
    for (const path of ['/api/v1/hourly', '/api/v1/admin/xnode/nodes/one/metrics', '/api/v1/admin/api-tokens']) {
      expect(responseCacheKey(path, { cache: 'no-store' })).toBeNull()
    }
    expect(responseCacheKey('/api/v1/dashboard')).not.toBeNull()
  })
  it('never persists member import keys', () => {
    expect(responseCacheKey('/api/v1/subscription')).toBeNull()
  })
  it('honors explicit no-store on financial reads and allows validated display snapshots', () => {
    expect(responseCacheKey('/api/v1/usage/breakdown', { cache: 'no-store' })).toBeNull()
    expect(responseCacheKey('/api/v1/usage/breakdown', { cache: 'no-cache' })).not.toBeNull()
    expect(responseCacheKey('/api/v1/balance')).toBeNull()
  })
  it('allows member display snapshots without allowing writes or payment capabilities', () => {
    for (const path of ['/api/v1/activity', '/api/v1/me/preferences', '/api/v1/me/group-member-tag', '/api/v1/me/internal-squads']) {
      expect(responseCacheKey(path)).not.toBeNull()
      expect(responseCacheKey(path, { method: 'POST' })).toBeNull()
    }
    expect(responseCacheKey('/api/v1/payments/orders/one')).toBeNull()
    expect(responseCacheKey('/api/v1/admin-access/links', { method: 'POST' })).toBeNull()
    expect(responseCacheKey('/api/v1/dashboard')).toBe('GET /api/v1/dashboard ')
  })
  it('does not cache or invalidate data for read-only firewall searches', () => {
    const path = '/api/v1/admin/xnode/firewall/records/search'
    expect(isReadRequest(path, 'POST')).toBe(true)
    expect(responseCacheKey(path, { method: 'POST', body: { filter: { op: 'and' }, limit: 50 } })).toBeNull()
  })
})
