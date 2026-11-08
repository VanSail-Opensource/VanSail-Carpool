import type { HourlyAccountView, HourlyEnrollment, HourlyPolicySnapshot, HourlyOffer } from '@/api/hourly'
import type { SquadProduct } from '@/api/types'
import { txbInputFromMinor } from '@/utils/format'
export const hourlyMoney = (minor: string) => ({ currency: 'TXB' as const, minor, display: `${txbInputFromMinor(minor)} TXB` })
export function hourlyFixture(params: URLSearchParams): { view: HourlyAccountView; policy: HourlyPolicySnapshot; squads: SquadProduct[]; offers: HourlyOffer[] } {
  const now = new Date().toISOString()
  const boundary = new Date(Math.floor(Date.now() / 3600000) * 3600000 + 3600000).toISOString()
  const midnight = new Date(new Date().setUTCHours(24, 0, 0, 0)).toISOString()
  const policy = { version: 'fixture-policy-1', policy: { salesEnabled: !params.has('paused'), dailyAllowanceBytes: '10000000000', reducedRateBytesPerSecond: '1250000', extraGBPriceTXBMinor: params.has('no-extra') ? '0' : '125', activationHours: 168, cooldownSeconds: 86400 } }
  const squads: SquadProduct[] = params.has('empty') ? [] : ['Tokyo Direct', 'Singapore Transit', 'Frankfurt'].map((name, index) => ({
    id: `squad-${index}`, groupId: `group-${index}`, name, description: '', features: [], billingMode: 'combo',
    profile: { type: 'international_network', portMbps: 1000, countryCode: ['JP', 'SG', 'DE'][index], upstreamCarriers: [] },
    paygPrice: hourlyMoney('100'), price: hourlyMoney('0'), visible: true, upstreamPresent: true,
    stockHeldByCurrentUser: false, stockLimit: 100, stockRemaining: 80, activationRequired: params.has('activation') && index === 0,
    accessibleNodes: [], createdAt: now, updatedAt: now,
  }))
  const offers = squads.map((squad, index) => ({ groupId: squad.groupId, enabled: true, hourlyPrice: hourlyMoney(String((index + 1) * 5)), revision: '1', createdAt: now, updatedAt: now }))
  const enrollment = (groupId: string): HourlyEnrollment => ({ id: `enrollment-${groupId}`, groupId, status: params.has('suspended') ? 'funding_suspended' : 'active', activatedAt: now, paidThrough: boundary, disabledAt: null, cooldownUntil: null, createdAt: now, updatedAt: now })
  const view: HourlyAccountView = {
    policy, balance: hourlyMoney('100000'), legacyBlocked: params.has('legacy'), extraPreference: { extraEnabled: params.has('extra'), acceptedExtraPriceTXBMinor: params.has('price-rise') ? '100' : '125', updatedAt: now },
    usage: { dayUTC: now.slice(0, 10), freeAllowanceBytes: '10000000000', freeUsedBytes: '2500000000', freeReservedBytes: '8192', extraFundedBytes: '2000000000', extraUsedBytes: '1000000000', extraReservedBytes: '32768', extraExpiresAt: midnight, reducedRateBytesPerSecond: '1250000', accountingPending: params.has('pending') },
    squads: squads.map((squad, index) => ({ squad, offer: offers[index]!, enrollment: (params.has('active') || params.has('suspended')) && index === 0 ? enrollment(squad.groupId) : null, ready: !params.has('incompatible'), reasonCode: params.has('incompatible') ? 'agent_incompatible' : '', cooldownUntil: null, requiredBalance: hourlyMoney(String((index + 1) * 5 * 168)) })),
  }
  if (params.has('cooldown') && view.squads[0]) {
    view.squads[0].ready = false
    view.squads[0].cooldownUntil = new Date(Date.now() + 86400000).toISOString()
  }
  return { view, policy, squads, offers }
}
