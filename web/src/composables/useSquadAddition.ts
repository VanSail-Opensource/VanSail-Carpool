import { usePricingQuote } from './usePricingQuote'
import { restoreRef } from '@/api/cache/restore'
import { computed, readonly, shallowRef, watch } from 'vue'

import { featuresApi, type CouponGrant } from '@/api/features'
import { isCouponGrantAvailable } from '@/utils/coupons'
import { ApiError, api } from '@/api/client'
import type { Catalog, Purchase, SquadProduct } from '@/api/types'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { notifyHaptic } from '@/utils/telegram'

interface SquadAdditionOptions {
  comboId?: () => string
  purchaseId: () => string
  activeSquadUuids: () => readonly string[]
}

export function useSquadAddition(options: SquadAdditionOptions) {
  const couponGrants = shallowRef<CouponGrant[]>([])
  const selectedCouponGrantId = shallowRef<string | null>(null)
  const catalog = shallowRef<Catalog | null>(null)
  const selectedSquadIds = shallowRef<string[]>([])
  const purchase = shallowRef<Purchase | null>(null)
  const loading = shallowRef(false)
  const purchasing = shallowRef(false)
  const needsBalance = shallowRef(false)
  const error = shallowRef<string | null>(null)
  let pendingAttempt: { fingerprint: string; key: string } | undefined
  const activeSquadSet = computed(() => new Set(options.activeSquadUuids()))
  const visibleSquads = computed(() => (catalog.value?.addons ?? []).filter((squad) =>
    squad.visible && squad.upstreamPresent && !activeSquadSet.value.has(squad.groupId),
  ))
  const selectedSquads = computed(() => visibleSquads.value.filter((squad) => selectedSquadIds.value.includes(squad.id)))
  const activationSquads = computed(() => selectedSquads.value.filter((squad) => squad.activationRequired))
  const eligibleCoupons = computed(() => couponGrants.value.filter(grant => {
    const coupon = grant.coupon
    return isCouponGrantAvailable(grant) && (coupon.kind === 'purchase_once' || coupon.kind === 'purchase_recurring') && (
      (!coupon.eligibleComboIds.length && !coupon.eligibleSquadIds.length) ||
      (options.comboId && coupon.eligibleComboIds.includes(options.comboId())) ||
      selectedSquads.value.some(squad => coupon.eligibleSquadIds.includes(squad.id) || coupon.eligibleSquadIds.includes(squad.groupId)))
  }))
  const fingerprint = computed(() => JSON.stringify([options.purchaseId(), [...selectedSquadIds.value].sort(), selectedCouponGrantId.value]))
  watch(selectedCouponGrantId, () => { clearQuote(); if (selectedSquadIds.value.length) void refreshQuote() })
  watch(eligibleCoupons, values => { if (selectedCouponGrantId.value && !values.some(grant => grant.id === selectedCouponGrantId.value)) selectedCouponGrantId.value = null })
  const { quote, quoting, quoteUsable, quoteStatus, clearQuote, refreshQuote } = usePricingQuote({
    selection: () => selectedSquadIds.value.length ? { purchaseId: options.purchaseId(), squads: [...selectedSquadIds.value], coupon: selectedCouponGrantId.value ?? undefined } : null,
    fingerprint: () => fingerprint.value,
    request: (selection, signal) => api.quotePurchaseAddons(selection.purchaseId, selection.squads, selection.coupon, signal),
    onSuccess: () => { error.value = null; needsBalance.value = false },
    onError: caught => { error.value = localizedError(caught, 'errors.quoteFailed'); if (!(caught instanceof ApiError && caught.code === 'PRICING_DECISION_PENDING')) notifyHaptic('error') },
  })

  function reset(): void {
    selectedCouponGrantId.value = null
    selectedSquadIds.value = []
    purchase.value = null
    error.value = null
    needsBalance.value = false
    clearQuote()
  }

  function isUnavailable(squad: SquadProduct): boolean {
    return squad.stockRemaining === 0 && !squad.stockHeldByCurrentUser
  }

  function toggleSquad(id: string): void {
    const squad = visibleSquads.value.find((candidate) => candidate.id === id)
    if (!squad || isUnavailable(squad)) return
    selectedSquadIds.value = selectedSquadIds.value.includes(id)
      ? selectedSquadIds.value.filter((value) => value !== id)
      : [...selectedSquadIds.value, id]
    clearQuote()
    error.value = null
  }

  async function load(): Promise<boolean> {
    loading.value = !restoreRef('/api/v1/catalog', catalog)
    error.value = null
    try {
      const [nextCatalog, wallet] = await Promise.all([api.getCatalog(), featuresApi.getCouponWallet().catch(() => null)])
      catalog.value = nextCatalog
      couponGrants.value = wallet?.items || []
      selectedSquadIds.value = selectedSquadIds.value.filter((id) => visibleSquads.value.some((squad) => squad.id === id))
      return true
    } catch (caught) {
      error.value = localizedError(caught, 'errors.catalogUnavailable')
      notifyHaptic('error')
      return false
    } finally {
      loading.value = false
    }
  }

  async function confirmPurchase(squadActivationCodes: Record<string, string>): Promise<boolean> {
    if (purchasing.value || !selectedSquadIds.value.length) return false
    if (!quoteUsable.value && !pendingAttempt) { await refreshQuote(); return false }
    purchasing.value = true
    error.value = null
    needsBalance.value = false
    try {
      const attemptFingerprint = JSON.stringify([options.purchaseId(), [...selectedSquadIds.value].sort(), selectedCouponGrantId.value, Object.entries(squadActivationCodes).sort(([a], [b]) => a.localeCompare(b))])
      if (pendingAttempt?.fingerprint !== attemptFingerprint) pendingAttempt = { fingerprint: attemptFingerprint, key: createUuid() }
      purchase.value = await api.addPurchaseAddons(options.purchaseId(), selectedSquadIds.value, pendingAttempt.key, squadActivationCodes, selectedCouponGrantId.value ?? undefined, quote.value?.quoteVersion)
      pendingAttempt = undefined
      notifyHaptic('success')
      return true
    } catch (caught) {
      if (caught instanceof ApiError && caught.code === 'PRICING_QUOTE_CHANGED') {
        clearQuote(); await refreshQuote(); error.value = t('pricing.quoteChanged')
      } else if (caught instanceof ApiError && caught.code === 'INSUFFICIENT_BALANCE') {
        needsBalance.value = true
        error.value = t('errors.purchaseBalance')
      } else {
        error.value = localizedError(caught, 'errors.purchaseFailed')
      }
      notifyHaptic('error')
      return false
    } finally {
      purchasing.value = false
    }
  }

  return {
    couponGrants: readonly(couponGrants), selectedCouponGrantId, eligibleCoupons,
    catalog: readonly(catalog), selectedSquadIds: readonly(selectedSquadIds), quote: readonly(quote), purchase: readonly(purchase),
    loading: readonly(loading), quoting: readonly(quoting), purchasing: readonly(purchasing), needsBalance: readonly(needsBalance), error: readonly(error),
    visibleSquads, selectedSquads, activationSquads, quoteUsable, quoteStatus, reset, toggleSquad, load, refreshQuote, confirmPurchase,
  }
}
