import type { HourlyAccountView, HourlyEnrollment, HourlyPolicySnapshot, HourlyOffer } from '@/api/hourly'
import type { SquadProduct } from '@/api/types'
import { txbInputFromMinor } from '@/utils/format'
export const hourlyMoney = (minor: string) => ({ currency: 'TXB' as const, minor, display: `${txbInputFromMinor(minor)} TXB` })
type FixtureOffer = HourlyOffer
export function hourlyFixture(params: URLSearchParams): { view: HourlyAccountView; policy: HourlyPolicySnapshot; squads: SquadProduct[]; offers: HourlyOffer[] } {
  const now = new Date().toISOString()
  const boundary = new Date(Math.floor(Date.now() / 3600000) * 3600000 + 3600000).toISOString()
  const midnight = new Date(new Date().setUTCHours(24, 0, 0, 0)).toISOString()
  // `extra` enables automatic purchases; `extra-off` overrides it in combined
  // audits so purchased credit can exist without authorizing new blocks.
  const extraEnabled = params.has('extra') && !params.has('extra-off')
  // `capacity` hides every free seat. `capacity-preparing` additionally models a
  // preparing enrollment whose next paid hour continues while a new activation
  // would be refused (shared stock, outcome 5).
  const capacity = params.has('capacity') || params.has('capacity-preparing')
  const offerHours = params.has('offer-hours') ? [24, 72, 0] : [0, 0, 0]
  const policy = { version: 'fixture-policy-1', policy: { salesEnabled: !params.has('paused'), dailyAllowanceBytes: '10000000000', reducedRateBytesPerSecond: '1250000', extraGBPriceTXBMinor: params.has('no-extra') ? '0' : '125', activationHours: 168, cooldownSeconds: 86400 } }
  const effectiveHours = (index: number): number => offerHours[index] || policy.policy.activationHours
  const squads: SquadProduct[] = params.has('empty') ? [] : ['Tokyo Direct', 'Singapore Transit', 'Frankfurt'].map((name, index) => ({
    id: `squad-${index}`, groupId: `group-${index}`, name, description: '', features: [], billingMode: 'combo',
    profile: { type: 'international_network', portMbps: 1000, countryCode: ['JP', 'SG', 'DE'][index], upstreamCarriers: [] },
    paygPrice: hourlyMoney('100'), price: hourlyMoney('0'), visible: true, upstreamPresent: true,
    stockHeldByCurrentUser: false, stockLimit: 100, stockRemaining: capacity ? 0 : 80, activationRequired: params.has('activation') && index === 0,
    accessibleNodes: [], createdAt: now, updatedAt: now,
  }))
  // The admin catalog lists Charge by traffic squads too; the hourly offer
  // editor must filter them out. Keep them out of the member hourly view, as
  // the server does.
  const paygSquad: SquadProduct = {
    id: 'squad-payg', groupId: 'group-payg', name: 'Metered Traffic', description: '', features: [], billingMode: 'payg',
    profile: { type: 'broadband', isp: 'Fixture ISP', portMbps: 1000, dynamic: false, location: 'United States' },
    paygPrice: hourlyMoney('100'), price: hourlyMoney('0'), visible: true, upstreamPresent: true,
    stockHeldByCurrentUser: false, stockLimit: 100, stockRemaining: 80, activationRequired: false,
    accessibleNodes: [], createdAt: now, updatedAt: now,
  }
  const adminSquads: SquadProduct[] = params.has('admin') ? [...squads, paygSquad] : squads
  const offers: FixtureOffer[] = squads.map((squad, index) => ({
    groupId: squad.groupId, enabled: true, hourlyPrice: hourlyMoney(String((index + 1) * 5)), revision: '1', createdAt: now, updatedAt: now,
    requiredHours: offerHours[index] ?? 0,
  }))
  const enrollmentStatus: HourlyEnrollment['status'] = params.has('suspended') ? 'funding_suspended' : params.has('capacity-preparing') ? 'preparing' : 'active'
  const enrollment = (groupId: string): HourlyEnrollment => ({ id: `enrollment-${groupId}`, groupId, status: enrollmentStatus, activatedAt: now, paidThrough: boundary, disabledAt: null, cooldownUntil: null, createdAt: now, updatedAt: now })
  const first = params.has('first')
  // `out-of-allowance` exhausts the free allowance so the reduced-speed state
  // shows. `allowance-zero` covers a zero allowance that already has traffic.
  // Purchased credit stays funded while the preference is off, so re-enabling
  // automatic purchases settles into the green purchased segments.
  const exhausted = params.has('out-of-allowance') || params.has('allowance-zero')
  const zeroAllowance = params.has('allowance-zero')
  const freeAllowance = zeroAllowance ? '0' : '10000000000'
  const freeUsed = first ? '0' : exhausted ? (zeroAllowance ? '2500000000' : freeAllowance) : '2500000000'
  // Existing purchased credit remains after the preference is turned off, so
  // the reduced-speed fixture still has a purchased segment to settle into
  // when automatic purchases are re-enabled during the audit.
  const extraFunded = first ? '0' : '2000000000'
  const extraUsed = first ? '0' : '1000000000'
  const view: HourlyAccountView = {
    policy, balance: hourlyMoney('100000'), legacyBlocked: params.has('legacy'), extraPreference: { extraEnabled, acceptedExtraPriceTXBMinor: params.has('price-rise') ? '100' : '125', updatedAt: now },
    usage: { dayUTC: now.slice(0, 10), freeAllowanceBytes: first ? '10000000000' : freeAllowance, freeUsedBytes: freeUsed, freeReservedBytes: first || exhausted ? '0' : '8192', extraFundedBytes: extraFunded, extraUsedBytes: extraUsed, extraReservedBytes: first || exhausted ? '0' : '32768', extraExpiresAt: midnight, reducedRateBytesPerSecond: '1250000', accountingPending: params.has('pending') },
    squads: squads.map((squad, index) => ({ squad, offer: offers[index]!, enrollment: !first && index === 0 && (params.has('active') || params.has('suspended') || params.has('capacity-preparing')) ? enrollment(squad.groupId) : null, ready: !params.has('incompatible') && !capacity, reasonCode: params.has('incompatible') ? 'agent_incompatible' : capacity ? 'capacity' : '', cooldownUntil: null, requiredBalance: hourlyMoney(String((index + 1) * 5 * effectiveHours(index))) })),
  }
  if (params.has('cooldown') && view.squads[0]) {
    view.squads[0].ready = false
    view.squads[0].cooldownUntil = new Date(Date.now() + 86400000).toISOString()
  }
  return { view, policy, squads: adminSquads, offers }
}
