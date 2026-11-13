import { restoreItems, restoreRef } from '@/api/cache/restore'
import { onMounted, onScopeDispose, readonly, shallowRef } from 'vue'

import type { AdminAffiliateView, AffiliateTier, CouponDefinition } from '@/api/features'
import { featuresApi } from '@/api/features'
import { localizedError } from '@/i18n'
import { cloneForEditing } from '@/utils/browserCompatibility'
import { createLatestRequest } from '@/utils/latestRequest'

export function useAdminAffiliates() {
  const configuration = shallowRef<AdminAffiliateView | null>(null)
  const coupons = shallowRef<CouponDefinition[]>([])
  const tiers = shallowRef<AffiliateTier[]>([])
  const loading = shallowRef(true)
  const saving = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  let disposed = false
  let hydrated = false

  async function load(): Promise<void> {
    if (disposed || saving.value) return
    const ticket = latest.begin()
    loading.value = hydrated || ![
      restoreRef('/api/v1/admin/affiliates', configuration),
      restoreItems('/api/v1/admin/coupons', coupons),
    ].every(Boolean)

    if (!hydrated && configuration.value) {
      tiers.value = cloneForEditing(configuration.value.tiers)
    }
    hydrated = true

    coupons.value = coupons.value.filter(
      (coupon) => coupon.active && coupon.kind.startsWith('purchase_'),
    )

    const originalTiers = JSON.stringify(tiers.value)
    error.value = null

    try {
      const [config, couponPage] = await Promise.all([
        featuresApi.getAdminAffiliates(),
        featuresApi.getAdminCoupons(),
      ])
      if (!latest.isCurrent(ticket)) return

      if (JSON.stringify(tiers.value) === originalTiers) {
        configuration.value = config
        tiers.value = cloneForEditing(config.tiers)
      }

      coupons.value = couponPage.items.filter(
        (coupon) => coupon.active && coupon.kind.startsWith('purchase_'),
      )
    } catch (caught) {
      if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'adminAffiliates.loadFailed')
    } finally {
      if (latest.isCurrent(ticket)) loading.value = false
    }
  }

  async function save(): Promise<void> {
    if (disposed || !configuration.value || saving.value || loading.value) return

    const ticket = latest.begin()
    const snapshot = cloneForEditing(tiers.value)
    const originalTiers = JSON.stringify(snapshot)
    saving.value = true
    error.value = null

    try {
      const saved = await featuresApi.saveAdminAffiliates(
        configuration.value.version,
        snapshot,
      )
      if (!latest.isCurrent(ticket)) return
      configuration.value = saved
      if (JSON.stringify(tiers.value) === originalTiers) tiers.value = cloneForEditing(saved.tiers)
    } catch (caught) {
      if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'adminAffiliates.saveFailed')
    } finally {
      if (latest.isCurrent(ticket)) saving.value = false
    }
  }

  function replace(next: AffiliateTier[]): void {
    if (!disposed) tiers.value = next
  }

  onMounted(() => void load())
  onScopeDispose(() => { disposed = true; latest.dispose(); configuration.value = null; tiers.value = []; coupons.value = [] })

  return {
    configuration: readonly(configuration),
    coupons: readonly(coupons),
    tiers: readonly(tiers),
    loading: readonly(loading),
    saving: readonly(saving),
    error: readonly(error),
    load,
    save,
    replace,
  }
}
