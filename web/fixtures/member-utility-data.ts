import type { AffiliateOverview, AffiliateReferralPage, EmbyOverview } from '@/api/features'
import type { Money, Session } from '@/api/types'
import type { UserPreferences } from '@/api/preferences'

export const utilityMoney = (minor: string): Money => ({ currency: 'TXB', minor, display: '' })

export function memberUtilityData(params: URLSearchParams) {
  const empty = params.has('empty')
  const at = (days: number) => new Date(Date.now() + days * 86_400_000).toISOString()
  const emby: EmbyOverview = {
    configured: !params.has('unconfigured'), setupPrice: utilityMoney('1800'),
    ratings: [{ name: 'PG', value: 7 }, { name: 'PG-13', value: 13 }, { name: 'R', value: 17 }],
    libraries: empty ? [] : [
      { id: 'cinema', name: 'Cinema · World Collection' },
      { id: 'documentary', name: 'Documentaries · Science and Nature' },
      { id: 'archive', name: 'InternationalCinemaArchive2026RestoredExtendedEditions' },
    ],
    account: empty ? null : { id: 'constructed-emby', username: 'sail_traveler', status: 'active', maxParentalRating: 13, disabledLibraryIds: ['archive'], retryable: false, updatedAt: at(0) },
  }
  const affiliates: AffiliateOverview = {
    ...(empty ? {} : { inviteLink: 'https://t.me/constructed_carpool_bot?start=invite_preview_only' }),
    totalCommission: utilityMoney(empty ? '0' : '123450'), registeredCount: empty ? 0 : 12,
    successfulCount: empty ? 0 : 8, conversionBps: empty ? 0 : 6667,
    tierProgress: {
      current: { id: 'traveler', name: 'Traveler', threshold: 0, enabled: true, commissionEnabled: true, commissionBps: 500, reward: { kind: 'none' } },
      next: { id: 'explorer', name: 'Explorer', threshold: 10, enabled: true, commissionEnabled: true, commissionBps: 800, reward: { kind: 'txb', txbMinor: 1800 } },
      successful: empty ? 0 : 8, remaining: empty ? 10 : 2, topTier: false,
    },
  }
  const referrals = (page: number): AffiliateReferralPage => ({
    page, pageSize: 3, total: empty ? 0 : 6, totalPages: empty ? 1 : 2,
    items: empty ? [] : [
      { firstName: page === 1 ? 'Yuki' : 'Hana', lastName: 'Nakamura', registeredAt: at(-18), status: 'successful', paybackAt: at(-16), commissionAmount: utilityMoney('3840') },
      { firstName: page === 1 ? 'Lin' : 'Chen', lastName: 'Wei', registeredAt: at(-12), status: 'pending' },
      { firstName: 'Alexandra-Maria', lastName: 'Fernández-Santiago', registeredAt: at(-8), status: 'successful', paybackAt: at(-7), commissionAmount: utilityMoney('12550') },
    ],
  })
  const session: Session = { authenticated: true, user: {
    id: 'constructed-utility-member', telegramId: '42', firstName: 'Sail', lastName: 'Traveler', telegramUsername: 'sail_traveler', displayName: 'Sail Traveler', username: null,
    role: 'admin', onboardingState: 'complete', groupJoined: true, channelJoined: true, policyAcceptedAt: at(-30), agreementRevision: 1, recoveryReason: '', displayCurrency: 'TXB', createdAt: at(-180), updatedAt: at(0),
  } }
  const preferences: UserPreferences = {
    paygEnabled: true, paygSquads: {}, activeCombo: true, showAroundTx: true, showActivity: true,
    includeNodePrices: true, showReferralUsername: true,
    notifications: { combos: true, traffic: true, money: true, activity: true, account: true },
  }
  return { emby, affiliates, referrals, session, preferences }
}
