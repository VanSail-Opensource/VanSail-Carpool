import type { Session } from '../types'
import type { RequestOptions } from '../http'

export interface PreloadResource { path: string; options?: RequestOptions }

export function memberResources(session: Session, locale: string): PreloadResource[] {
  const onboarding = { path: '/api/v1/onboarding/content', options: { query: { locale } } }
  if (session.user.onboardingState !== 'complete') return [onboarding]
  const end = new Date()
  const start = new Date(end)
  start.setUTCDate(start.getUTCDate() - 6)
  return [
    ...['dashboard', 'catalog', 'balance', 'statistics', 'statistics/nodes', 'activity',
      'affiliates', 'coupons/wallet', 'questionnaires/active', 'emby/account',
      'me/abuse-records', 'subscription/ip-blocks', 'me/traffic-reset-automation',
      'community/membership/check'].map(path => ({ path: `/api/v1/${path}` })),
    { path: '/api/v1/affiliates/referrals', options: { query: { page: 1 } } },
    { path: '/api/v1/dashboard/node-usage', options: { query: { start: start.toISOString().slice(0, 10), end: end.toISOString().slice(0, 10) } } },
    { path: '/api/v1/community/membership/check', options: { method: 'POST' } },
    onboarding,
  ]
}
