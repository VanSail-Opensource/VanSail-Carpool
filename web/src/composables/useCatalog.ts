import { computed, getCurrentInstance, onMounted, onScopeDispose, readonly, shallowRef, watch } from 'vue'
import { api, ApiError } from '@/api/client'
import { featuresApi, type CouponGrant } from '@/api/features'
import type { Catalog, Combo, Purchase, SquadProduct } from '@/api/types'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { notifyHaptic } from '@/utils/telegram'
import { useSessionStore } from '@/stores/session'
import { isCouponGrantAvailable } from '@/utils/coupons'
import { clearCatalogDraft, writeCatalogDraft } from '@/composables/catalogDraft'
import { loadCatalogData } from '@/composables/catalogLoader'
import { useCatalogQuote } from '@/composables/catalogQuoteState'
import { createLatestRequest } from '@/utils/latestRequest'
export function useCatalog() {
  const sessionStore = useSessionStore()
  const catalog = shallowRef<Catalog | null>(null)
  const balance = shallowRef<import('@/api/types').Money | null>(null)
  const loading = shallowRef(true)
  const purchasing = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const purchase = shallowRef<Purchase | null>(null)
  const selectedComboId = shallowRef<string | null>(null)
  const selectedSquadIds = shallowRef<string[]>([])
  const couponGrants = shallowRef<CouponGrant[]>([])
  const selectedCouponGrantId = shallowRef<string | null>(null)
  const couponDiscarding = shallowRef(false)
  const needsBalance = shallowRef(false)
  const purchaseIdempotencyKey = shallowRef<string | null>(null)
  const draftRestored = shallowRef(false)
  const latestLoad = createLatestRequest()
  function persistDraft(): void {
    if (purchase.value) return
    writeCatalogDraft(sessionStore.user?.id, {
      comboId: selectedComboId.value ?? undefined,
      squadIds: selectedSquadIds.value,
      couponGrantId: selectedCouponGrantId.value,
    })
  }
  const visibleCombos = computed(() => catalog.value?.combos.filter((combo) => combo.active) ?? [])
  const selectedCombo = computed<Combo | undefined>(() => visibleCombos.value.find((combo) => combo.id === selectedComboId.value))
  const includedSquadIds = computed(() => selectedCombo.value?.includedSquads.map((squad) => squad.id) ?? [])
  const visibleSquads = computed(() => {
    const squads = [
      ...(selectedCombo.value?.includedSquads ?? []),
      ...(catalog.value?.addons.filter((squad) => squad.visible && squad.upstreamPresent) ?? []),
    ]
    return squads.filter((squad, index) => squads.findIndex((candidate) => candidate.id === squad.id) === index)
  })
  const selectedSquads = computed<SquadProduct[]>(() => visibleSquads.value.filter((squad) => selectedSquadIds.value.includes(squad.id)))
  const activationSquads = computed<SquadProduct[]>(() => visibleSquads.value.filter((squad) => squad.activationRequired && (includedSquadIds.value.includes(squad.id) || selectedSquadIds.value.includes(squad.id))))
  const eligibleCoupons = computed(() => couponGrants.value.filter((grant) => {
    const coupon = grant.coupon
    if (!selectedCombo.value || !isCouponGrantAvailable(grant)) return false
    const unrestricted = coupon.eligibleComboIds.length === 0 && coupon.eligibleSquadIds.length === 0
    const comboEligible = coupon.eligibleComboIds.includes(selectedCombo.value.id)
    const squadEligible = selectedSquadIds.value.some((id) => coupon.eligibleSquadIds.includes(id))
    if (!unrestricted && !comboEligible && !squadEligible) return false
    return coupon.kind === 'purchase_once' || coupon.kind === 'purchase_recurring'
  }))
  const purchaseFingerprint = computed(() => JSON.stringify([
    selectedComboId.value,
    [...selectedSquadIds.value].sort(),
    selectedCouponGrantId.value,
  ]))
  const { quote, quoting, quoteUsable, quoteStatus, clearQuote, refreshQuote: fetchQuote } = useCatalogQuote({
    selection: () => selectedCombo.value ? {
      comboId: selectedCombo.value.id,
      squadProductIds: [...selectedSquadIds.value],
      couponGrantId: selectedCouponGrantId.value ?? undefined,
    } : null,
    fingerprint: () => purchaseFingerprint.value,
    onSuccess: () => { error.value = null },
    onError: (caught) => {
      error.value = localizedError(caught, 'errors.quoteFailed')
      if (!(caught instanceof ApiError && caught.code === 'PRICING_DECISION_PENDING')) notifyHaptic('error')
    },
  })
  watch(purchaseFingerprint, () => {
    purchaseIdempotencyKey.value = null
    if (!purchase.value) clearQuote()
    persistDraft()
  })
  function refreshQuote(): Promise<boolean> {
    error.value = null
    needsBalance.value = false
    return fetchQuote()
  }
  async function load(): Promise<void> {
    const applied = await loadCatalogData({ loading, error, catalog, balance, couponGrants, draftRestored, selectedComboId, selectedSquadIds, selectedCouponGrantId, userID: () => sessionStore.user?.id, latest: latestLoad })
    if (!applied) return
    couponGrants.value = couponGrants.value.filter((grant) => isCouponGrantAvailable(grant))
    if (!eligibleCoupons.value.some((grant) => grant.id === selectedCouponGrantId.value)) selectedCouponGrantId.value = null
    selectedSquadIds.value = selectedSquadIds.value.filter((id) => !paidAddonIsFull(id))
  }
  function selectCombo(id: string): void {
    selectedComboId.value = id
    const included = visibleCombos.value.find((combo) => combo.id === id)?.includedSquads.map((squad) => squad.id) ?? []
    selectedSquadIds.value = selectedSquadIds.value.filter((squadId) => !included.includes(squadId))
    if (!eligibleCoupons.value.some((coupon) => coupon.id === selectedCouponGrantId.value)) selectedCouponGrantId.value = null
    purchase.value = null
  }
  function toggleSquad(id: string): void {
    if (includedSquadIds.value.includes(id) || paidAddonIsFull(id)) return
    selectedSquadIds.value = selectedSquadIds.value.includes(id)
      ? selectedSquadIds.value.filter((value) => value !== id)
      : [...selectedSquadIds.value, id]
    if (!eligibleCoupons.value.some((coupon) => coupon.id === selectedCouponGrantId.value)) selectedCouponGrantId.value = null
  }
  function paidAddonIsFull(id: string): boolean { return !includedSquadIds.value.includes(id) && visibleSquads.value.some((squad) => squad.id === id && squad.stockRemaining === 0 && !squad.stockHeldByCurrentUser) }
  async function discardCoupon(grantID: string): Promise<boolean> {
    if (couponDiscarding.value || !couponGrants.value.some((grant) => grant.id === grantID)) return false
    couponDiscarding.value = true
    error.value = null
    try {
      await featuresApi.discardCouponWalletGrant(grantID)
      couponGrants.value = couponGrants.value.filter((grant) => grant.id !== grantID)
      if (selectedCouponGrantId.value === grantID) selectedCouponGrantId.value = null
      notifyHaptic('success')
      return true
    } catch (caught) {
      error.value = localizedError(caught, 'coupons.discardFailed')
      notifyHaptic('error')
      return false
    } finally {
      couponDiscarding.value = false
    }
  }
  async function confirmPurchase(squadActivationCodes: Record<string, string> = {}): Promise<boolean> {
    if (!selectedCombo.value || purchase.value || purchasing.value) return false
    if (!quoteUsable.value && !purchaseIdempotencyKey.value) { await refreshQuote(); return false }
    purchasing.value = true
    error.value = null
    needsBalance.value = false
    try {
      purchaseIdempotencyKey.value ??= createUuid()
      purchase.value = await api.createPurchase(
        selectedCombo.value.id,
        selectedSquadIds.value,
        selectedCouponGrantId.value ?? undefined,
        purchaseIdempotencyKey.value,
        squadActivationCodes,
        quote.value?.quoteVersion,
      )
      notifyHaptic('success')
      purchaseIdempotencyKey.value = null
      selectedSquadIds.value = []
      selectedCouponGrantId.value = null
      clearCatalogDraft(sessionStore.user?.id)
      await load()
      return true
    } catch (caught) {
      if (caught instanceof ApiError && caught.code === 'PRICING_QUOTE_CHANGED') {
        clearQuote(); await fetchQuote(); error.value = t('pricing.quoteChanged')
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
  if (getCurrentInstance()) onMounted(() => void load())
  if (getCurrentInstance()) onScopeDispose(latestLoad.dispose)
  return {
    catalog: readonly(catalog), balance: readonly(balance),
    loading: readonly(loading), purchasing: readonly(purchasing),
    quoting: readonly(quoting), quoteStatus,
    error: readonly(error), purchase: readonly(purchase),
    quote: readonly(quote), quoteUsable,
    selectedComboId,
    selectedSquadIds: readonly(selectedSquadIds),
    selectedCouponGrantId,
    needsBalance: readonly(needsBalance),
    visibleCombos,
    visibleSquads,
    selectedCombo,
    selectedSquads,
    activationSquads,
    includedSquadIds,
    couponGrants: readonly(couponGrants),
    eligibleCoupons,
    couponDiscarding: readonly(couponDiscarding),
    load,
    selectCombo,
    toggleSquad,
    refreshQuote,
    discardCoupon,
    confirmPurchase,
  }
}
