import { describe, expect, it } from 'vitest'

import type { components, paths } from './generated'

type IsRequired<T, Key extends keyof T> = object extends Pick<T, Key> ? false : true
type Squad = components['schemas']['SquadProduct']
type Node = components['schemas']['CatalogNode']

describe('generated OpenAPI release contract', () => {
  it('keeps retired reset automation absent and nested nodes required', () => {
    const contractChecks: [
      '/api/v1/me/traffic-reset-automation' extends keyof paths ? false : true,
      IsRequired<Squad, 'accessibleNodes'>,
      null extends Node['providerName'] ? true : false,
    ] = [true, true, true]

    expect(contractChecks).toEqual([true, true, true])
  })

  it('represents all current native lifecycle states and shared browser selection', () => {
    type Synchronization = components['schemas']['AdminUserSummary']['synchronization']['status']
    const statuses: Synchronization[] = ['synchronized', 'pending', 'failed', 'not_provisioned']
    type Headers = NonNullable<paths['/api/v1/me']['get']['parameters']['header']>
    const headers: Headers = { 'X-TXC-Session-Scope': 'browser' }

    expect(statuses).toHaveLength(4)
    expect(headers['X-TXC-Session-Scope']).toBe('browser')
  })

  it('requires a typed moderation edit instead of an unknown payload', () => {
    type Update = components['schemas']['PMModerationUpdate']
    const checks: [
      unknown extends Update ? false : true,
      Record<string, never> extends Update ? false : true,
      { blocked: string } extends Update ? false : true,
      { blocked: boolean } extends Update ? true : false,
      { muted: boolean } extends Update ? true : false,
    ] = [true, true, true, true, true]

    expect(checks.every(Boolean)).toBe(true)
  })

  it('retains the required duration in preview and creation types', () => {
    type Targets = { comboIds: string[]; addonSquadUuids: string[] }
    type Preview = components['schemas']['BulkExtensionRequest']
    type Create = components['schemas']['BulkExtensionCreateRequest']
    const checks: [
      Targets extends Preview ? false : true,
      Targets & { durationMinutes: number } extends Preview ? true : false,
      Targets & { days: number } extends Preview ? true : false,
      Targets & { reason: string } extends Create ? false : true,
      Targets & { reason: string; durationMinutes: number } extends Create ? true : false,
      Targets & { durationMinutes: number } extends Create ? false : true,
    ] = [true, true, true, true, true, true]

    expect(checks.every(Boolean)).toBe(true)
  })

  it('uses affiliate wire values for generated reward discriminators', () => {
    const kinds: [
      components['schemas']['AffiliateNoReward']['kind'],
      components['schemas']['AffiliateCouponReward']['kind'],
      components['schemas']['AffiliateTXBReward']['kind'],
      components['schemas']['AffiliateExtensionReward']['kind'],
    ] = ['none', 'coupon', 'txb', 'subscription_extension']

    expect(kinds).toEqual(['none', 'coupon', 'txb', 'subscription_extension'])
  })
})
