import type { RouteLocationNormalized } from 'vue-router'
import { describe, expect, it } from 'vitest'

import type { User } from '@/api/types'
import { resolveProtectedRoute } from './guards'

function route(path: string): RouteLocationNormalized {
  return { path } as RouteLocationNormalized
}

function user(overrides: Partial<User> = {}): User {
  return {
    id: 'user-1',
    telegramId: '42',
    firstName: 'Mira',
    lastName: 'Lin',
    telegramUsername: 'miralin',
    username: 'mira',
    role: 'user',
    onboardingState: 'complete',
    groupJoined: true,
    channelJoined: true,
    policyAcceptedAt: '2026-08-07T00:00:00Z',
    createdAt: '2026-08-07T00:00:00Z',
    updatedAt: '2026-08-07T00:00:00Z',
    ...overrides,
    agreementRevision: overrides.agreementRevision ?? 1,
    recoveryReason: overrides.recoveryReason ?? '',
  }
}

describe('protected route decisions', () => {
  it('resumes incomplete onboarding', () => {
    expect(resolveProtectedRoute(route('/catalog'), user({ onboardingState: 'username' }))).toBe('/onboarding')
  })

  it('keeps completed users out of onboarding', () => {
    expect(resolveProtectedRoute(route('/onboarding'), user())).toBe('/home')
  })

  it('rejects non-admin users from every admin section', () => {
    expect(resolveProtectedRoute(route('/admin/users'), user())).toBe('/home')
  })

  it('sends a first-launch admin to the OAuth/access exception', () => {
    expect(resolveProtectedRoute(route('/'), user({ role: 'admin', onboardingState: 'intro' }))).toBe('/settings/admin-access')
  })

  it('lets an un-onboarded admin start optional signup from the dashboard', () => {
    expect(resolveProtectedRoute(route('/onboarding'), user({ role: 'admin', onboardingState: 'username' }))).toBeNull()
  })

  it('keeps an un-onboarded admin out of product routes', () => {
    expect(resolveProtectedRoute(route('/catalog'), user({ role: 'admin', onboardingState: 'agreement' }))).toBe('/settings/admin-access')
  })

  it.each(['/admin/settings', '/admin/users', '/admin/users/user-1', '/admin/database'])('bridges legacy Mini App admin route %s to browser access', (path) => {
    expect(resolveProtectedRoute(route(path), user({ role: 'admin', onboardingState: 'username' }))).toBe('/settings/admin-access')
    expect(resolveProtectedRoute(route(path), user({ role: 'admin', onboardingState: 'complete' }))).toBe('/settings/admin-access')
  })

  it('permits only the access exception before administrator onboarding', () => {
    expect(resolveProtectedRoute(route('/settings/admin-access'), user({ role: 'admin', onboardingState: 'intro' }))).toBeNull()
    expect(resolveProtectedRoute(route('/settings/admin-access'), user())).toBe('/home')
    expect(resolveProtectedRoute(route('/settings/admin-access'), user({ onboardingState: 'intro' }))).toBe('/home')
  })

  it('lets a completed admin use user-side routes', () => {
    const completedAdmin = user({ role: 'admin', onboardingState: 'complete' })
    expect(resolveProtectedRoute(route('/'), completedAdmin)).toBe('/home')
    expect(resolveProtectedRoute(route('/catalog'), completedAdmin)).toBeNull()
  })

  it('keeps a completed admin out of signup', () => {
    expect(resolveProtectedRoute(route('/onboarding'), user({ role: 'admin', onboardingState: 'complete' }))).toBe('/home')
  })
})
