import type { Session } from '../types'
import type { RequestOptions } from '../http'

export interface PreloadResource { path: string; options?: RequestOptions }

export function memberResources(session: Session, locale: string): PreloadResource[] {
  const onboarding = { path: '/api/v1/onboarding/content', options: { query: { locale } } }
  if (session.user.onboardingState !== 'complete') return [onboarding]
  return [
    // Prioritize the current member surfaces while their lazy chunks load.
    ...['dashboard', 'usage/breakdown', 'payg', 'hourly', 'xnode/diy',
      'activity', 'me/preferences', 'me/group-member-tag', 'me/internal-squads',
      'affiliates', 'emby/account',
      'community/membership/check'].map(path => ({ path: `/api/v1/${path}` })),
    { path: '/api/v1/affiliates/referrals', options: { query: { page: 1 } } },
    { path: '/api/v1/community/membership/check', options: { method: 'POST' } },
  ]
}
