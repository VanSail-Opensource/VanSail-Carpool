import { usePricingQuote } from './usePricingQuote'
import { restoreRef } from '@/api/cache/restore'
import { computed, onScopeDispose, readonly, shallowRef, watch } from 'vue'

import { featuresApi, type CouponGrant } from '@/api/features'
import { isCouponGrantAvailable } from '@/utils/coupons'
import { ApiError, api } from '@/api/client'
import type { Catalog, Purchase, SquadProduct } from '@/api/types'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { notifyHaptic } from '@/utils/telegram'
import { createLatestRequest } from '@/utils/latestRequest'

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
  const pendingAttempts = new Map<string, string>()
  const loads = createLatestRequest()
  const mutations = createLatestRequest()
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
    loads.invalidate()
    mutations.invalidate()
    loading.value = false
    purchasing.value = false
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
    const ticket = loads.begin()
    loading.value = !restoreRef('/api/v1/catalog', catalog)
    error.value = null
    try {
      const [nextCatalog, wallet] = await Promise.all([api.getCatalog(), featuresApi.getCouponWallet().catch(() => null)])
      if (!loads.isCurrent(ticket)) return false
      catalog.value = nextCatalog
      couponGrants.value = wallet?.items || []
      selectedSquadIds.value = selectedSquadIds.value.filter((id) => visibleSquads.value.some((squad) => squad.id === id))
      return true
    } catch (caught) {
      if (!loads.isCurrent(ticket)) return false
      error.value = localizedError(caught, 'errors.catalogUnavailable')
      notifyHaptic('error')
      return false
    } finally {
      if (loads.isCurrent(ticket)) loading.value = false
    }
  }

  async function confirmPurchase(squadActivationCodes: Record<string, string>): Promise<boolean> {
    if (purchasing.value || !selectedSquadIds.value.length) return false
    const attemptFingerprint = fingerprint.value
    if (!quoteUsable.value && !pendingAttempts.has(attemptFingerprint)) { await refreshQuote(); return false }
    const ticket = mutations.begin()
    const purchaseId = options.purchaseId()
    purchasing.value = true
    error.value = null
    needsBalance.value = false
    try {
      const key = pendingAttempts.get(attemptFingerprint) ?? createUuid()
      pendingAttempts.set(attemptFingerprint, key)
      const result = await api.addPurchaseAddons(purchaseId, [...selectedSquadIds.value], key, squadActivationCodes, selectedCouponGrantId.value ?? undefined, quote.value?.quoteVersion)
      if (!mutations.isCurrent(ticket) || fingerprint.value !== attemptFingerprint) return false
      purchase.value = result
      pendingAttempts.delete(attemptFingerprint)
      notifyHaptic('success')
      return true
    } catch (caught) {
      if (!mutations.isCurrent(ticket) || fingerprint.value !== attemptFingerprint) return false
      if (caught instanceof ApiError && caught.code === 'PRICING_QUOTE_CHANGED') {
        clearQuote(); await refreshQuote()
        if (!mutations.isCurrent(ticket) || fingerprint.value !== attemptFingerprint) return false
        error.value = t('pricing.quoteChanged')
      } else if (caught instanceof ApiError && caught.code === 'INSUFFICIENT_BALANCE') {
        needsBalance.value = true
        error.value = t('errors.purchaseBalance')
      } else {
        error.value = localizedError(caught, 'errors.purchaseFailed')
      }
      notifyHaptic('error')
      return false
    } finally {
      if (mutations.isCurrent(ticket)) purchasing.value = false
    }
  }

  onScopeDispose(() => { loads.dispose(); mutations.dispose(); pendingAttempts.clear() })

  return {
    couponGrants: readonly(couponGrants), selectedCouponGrantId, eligibleCoupons,
    catalog: readonly(catalog), selectedSquadIds: readonly(selectedSquadIds), quote: readonly(quote), purchase: readonly(purchase),
    loading: readonly(loading), quoting: readonly(quoting), purchasing: readonly(purchasing), needsBalance: readonly(needsBalance), error: readonly(error),
    visibleSquads, selectedSquads, activationSquads, quoteUsable, quoteStatus, reset, toggleSquad, load, refreshQuote, confirmPurchase,
  }
}
