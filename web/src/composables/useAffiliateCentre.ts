import { computed, onMounted, onScopeDispose, readonly, shallowRef } from 'vue'

import type { AffiliateOverview, AffiliateReferralPage } from '@/api/features'
import { featuresApi } from '@/api/features'
import { restoreRef } from '@/api/cache/restore'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useAffiliateCentre() {
  const overview = shallowRef<AffiliateOverview | null>(null)
  const referrals = shallowRef<AffiliateReferralPage | null>(null)
  const page = shallowRef(1)
  const loading = shallowRef(true)
  const referralsLoading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  const latestPage = createLatestRequest()

  async function load(): Promise<void> {
    const token = latest.begin()
    const pageToken = latestPage.begin()
    loading.value = ![
      restoreRef('/api/v1/affiliates', overview),
      restoreRef('/api/v1/affiliates/referrals', referrals, { query: { page: page.value } }),
    ].every(Boolean)
    error.value = null
    try {
      const [summary, rows] = await Promise.all([featuresApi.getAffiliates(), featuresApi.getAffiliateReferrals(page.value)])
      if (!latest.isCurrent(token)) return
      overview.value = summary
      if (latestPage.isCurrent(pageToken)) referrals.value = rows
    } catch (caught) {
      if (latest.isCurrent(token) && latestPage.isCurrent(pageToken)) error.value = localizedError(caught, 'affiliates.loadFailed')
    } finally {
      if (latest.isCurrent(token)) loading.value = false
      if (latestPage.isCurrent(pageToken)) referralsLoading.value = false
    }
  }

  async function setPage(next: number): Promise<void> {
    if (!Number.isInteger(next) || next < 1 || (next === page.value && (referralsLoading.value || referrals.value?.page === next))) return
    const token = latestPage.begin()
    page.value = next
    error.value = null
    referralsLoading.value = !restoreRef('/api/v1/affiliates/referrals', referrals, { query: { page: next } })
    try {
      const rows = await featuresApi.getAffiliateReferrals(next)
      if (latestPage.isCurrent(token)) referrals.value = rows
    } catch (caught) {
      if (latestPage.isCurrent(token)) error.value = localizedError(caught, 'affiliates.referralsFailed')
    } finally {
      if (latestPage.isCurrent(token)) referralsLoading.value = false
    }
  }

  onMounted(() => void load())
  onScopeDispose(() => { latest.dispose(); latestPage.dispose() })
  return { overview: computed(() => overview.value), referrals: computed(() => referrals.value), page: computed(() => page.value), loading: readonly(loading), referralsLoading: readonly(referralsLoading), error: readonly(error), load, setPage }
}
