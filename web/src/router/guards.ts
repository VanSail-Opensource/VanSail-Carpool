import type { RouteLocationNormalized } from 'vue-router'

import type { User } from '@/api/types'

export function resolveProtectedRoute(to: RouteLocationNormalized, user: User | null): string | null {
  if (!user) return null

  const wantsAdmin = to.path.startsWith('/admin')
  const wantsAccess = to.path === '/settings/admin-access'
  if (user.role === 'admin') {
    if (wantsAdmin) return '/settings/admin-access'
    if (wantsAccess) return null
    if (user.onboardingState === 'complete') return to.path === '/onboarding' || to.path === '/' ? '/home' : null
    return to.path === '/onboarding' ? null : '/settings/admin-access'
  }

  if (wantsAdmin || wantsAccess) return '/home'

  if (user.onboardingState !== 'complete' && !wantsAdmin && to.path !== '/onboarding') {
    return '/onboarding'
  }

  if (user.onboardingState === 'complete' && to.path === '/onboarding') return '/home'
  if (to.path === '/') return user.onboardingState === 'complete' ? '/home' : '/onboarding'

  return null
}
