import { describe, expect, it } from 'vitest'
import type { Session } from '../types'
import { memberResources } from './preloadRoutes'
import { isReadRequest } from './policy'

describe('initial page preload inventory', () => {
  const session = (role: string, onboardingState: string) => ({ user: { role, onboardingState } }) as Session

  it('keeps incomplete members on localized onboarding data', () => {
    expect(memberResources(session('member', 'agreement'), 'zh-CN')).toEqual([
      { path: '/api/v1/onboarding/content', options: { query: { locale: 'zh-CN' } } },
    ])
  })

  it('covers member pages with their initial queries and no admin resources', () => {
    const resources = memberResources(session('member', 'complete'), 'en')
    const paths = resources.map(resource => resource.path)
    expect(paths).toEqual(expect.arrayContaining([
      '/api/v1/dashboard', '/api/v1/activity',
      '/api/v1/affiliates', '/api/v1/emby/account', '/api/v1/questionnaires/active',
      '/api/v1/subscription/ip-blocks',
    ]))
    expect(paths.some(path => path.includes('/admin/') || path.includes('/statistics'))).toBe(false)
    for (const path of ['/api/v1/payg', '/api/v1/catalog', '/api/v1/coupons/wallet']) expect(paths).not.toContain(path)
    expect(resources.find(resource => resource.path.endsWith('/referrals'))?.options?.query).toEqual({ page: 1 })
    expect(resources.every(resource => isReadRequest(resource.path, resource.options?.method ?? 'GET'))).toBe(true)
  })

  it('loads only member resources for an onboarded Mini App administrator', () => {
    const resources = memberResources(session('admin', 'complete'), 'en')
    expect(resources).toEqual(memberResources(session('user', 'complete'), 'en'))
    expect(resources.some(resource => resource.path.includes('/admin/') || resource.path.includes('/admin-access/'))).toBe(false)
    expect(resources.every(resource => isReadRequest(resource.path, resource.options?.method ?? 'GET'))).toBe(true)
  })

  it('keeps an unonboarded administrator on optional onboarding content', () => {
    expect(memberResources(session('admin', 'intro'), 'en')).toEqual([
      { path: '/api/v1/onboarding/content', options: { query: { locale: 'en' } } },
    ])
  })
})
