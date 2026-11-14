import { api } from '@/api/client'
import { featuresApi } from '@/api/features'
import { ApiError } from '@/api/http'
import type { User } from '@/api/types'

// Used only by the constructed member shell. No real invites or Emby writes.
export function installAroundTxFixture(params: URLSearchParams, user: () => User) {
  const trace = { affiliateReads: 0, communityReads: 0, embyReads: 0 }
  const pause = async () => {
    if (params.has('around-slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('around-slow')) || 1500))
    if (params.has('around-error')) throw new ApiError(503, { code: 'UNAVAILABLE', message: '' })
  }
  featuresApi.getAffiliates = async () => {
    trace.affiliateReads++; await pause()
    return {
      inviteLink: 'https://t.me/fixture_bot?start=fixture',
      totalCommission: { currency: 'TXB', minor: '1200', display: '12.00 TXB' },
      registeredCount: 3, successfulCount: 2, conversionBps: 6667,
      tierProgress: {
        current: { id: 'fixture-tier', name: 'Member', threshold: 0, enabled: true, commissionEnabled: true, commissionBps: 500, reward: { kind: 'none' } },
        successful: 2, remaining: 0, topTier: true,
      },
    }
  }
  featuresApi.getAffiliateReferrals = async page => {
    await pause()
    return { items: [], page, pageSize: 25, total: 0, totalPages: 1 }
  }
  api.checkCommunityMembership = async () => {
    trace.communityReads++; await pause()
    return { user: { ...user(), groupJoined: true, channelJoined: true }, activeCombo: true, groupJoined: true, channelJoined: true }
  }
  featuresApi.getEmby = async () => {
    trace.embyReads++; await pause()
    return {
      configured: true, setupPrice: { currency: 'TXB', minor: '100', display: '1.00 TXB' },
      ratings: [], libraries: [],
      account: { id: 'fixture-emby', username: 'fixture_member', status: 'active', maxParentalRating: null, disabledLibraryIds: [], retryable: false, updatedAt: new Date().toISOString() },
    }
  }
  const rejectWrite = async () => { throw new ApiError(503, { code: 'UNAVAILABLE', message: '' }) }
  api.createCommunityInvite = rejectWrite
  featuresApi.setupEmby = rejectWrite
  featuresApi.updateEmbyPreferences = rejectWrite
  featuresApi.changeEmbyPassword = rejectWrite
  Object.assign(window, { __aroundTxAudit: trace })
}
