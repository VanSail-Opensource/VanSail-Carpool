import { computed, onMounted, onScopeDispose, reactive, shallowRef } from 'vue'
import { api } from '@/api/client'
import { restoreItems } from '@/api/cache/restore'
import { featuresApi, type CouponDefinition } from '@/api/features'
import type { Combo, SquadProduct } from '@/api/types'
import { localizedError, t } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'
import { couponDraft, couponPayload } from './couponDraft'

export function useAdminCoupons() {
  const items = shallowRef<CouponDefinition[]>([])
  const loading = shallowRef(true)
  const refreshing = shallowRef(false)
  const busy = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const combos = shallowRef<Combo[]>([])
  const squads = shallowRef<SquadProduct[]>([])
  const optionsLoading = shallowRef(false)
  const optionsError = shallowRef<string | null>(null)
  const editingId = shallowRef<string | null | undefined>(undefined)
  const deactivating = shallowRef<CouponDefinition | null>(null)
  const draft = reactive(couponDraft())
  const comboItems = computed(() => combos.value.map(combo => ({ value: combo.id, label: combo.name })))
  const squadItems = computed(() => squads.value.map(squad => ({ value: squad.id, label: squad.name })))
  const requests = createLatestRequest()
  const optionRequests = createLatestRequest()
  let disposed = false
  let hydrated = false
  let optionsHydrated = false
  let original: CouponDefinition | null = null

  async function load(): Promise<void> {
    if (disposed || busy.value) return
    const ticket = requests.begin()
    if (!hydrated) {
      loading.value = !restoreItems('/api/v1/admin/coupons', items)
      hydrated = true
    }
    refreshing.value = true
    error.value = null
    try {
      const result = await featuresApi.getAdminCoupons()
      if (requests.isCurrent(ticket)) items.value = result.items
    } catch (caught) {
      if (requests.isCurrent(ticket)) error.value = localizedError(caught, 'adminCoupons.loadFailed')
    } finally {
      if (requests.isCurrent(ticket)) { loading.value = false; refreshing.value = false }
    }
  }

  async function loadOptions(): Promise<void> {
    if (disposed || optionsLoading.value) return
    const ticket = optionRequests.begin()
    if (!optionsHydrated) {
      restoreItems('/api/v1/admin/combos', combos)
      restoreItems('/api/v1/admin/squad-products', squads)
      optionsHydrated = true
    }
    optionsLoading.value = true
    optionsError.value = null
    try {
      const [comboResponse, squadResponse] = await Promise.all([
        api.getAdminResource<{ items: Combo[] }>('combos'),
        api.getAdminResource<{ items: SquadProduct[] }>('squad-products'),
      ])
      if (!optionRequests.isCurrent(ticket)) return
      combos.value = comboResponse.items
      squads.value = squadResponse.items
    } catch (caught) {
      if (optionRequests.isCurrent(ticket)) optionsError.value = localizedError(caught, 'adminCoupons.optionsFailed')
    } finally {
      if (optionRequests.isCurrent(ticket)) optionsLoading.value = false
    }
  }

  function edit(coupon?: CouponDefinition): void {
    if (disposed || busy.value || loading.value) return
    original = coupon ? { ...coupon, eligibleComboIds: [...coupon.eligibleComboIds], eligibleSquadIds: [...coupon.eligibleSquadIds] } : null
    editingId.value = coupon?.id ?? null
    error.value = null
    Object.assign(draft, couponDraft(coupon))
  }

  function close(): void { if (!busy.value) { editingId.value = undefined; original = null } }
  function requestDeactivation(coupon: CouponDefinition): void {
    const current = items.value.find(item => item.id === coupon.id)
    if (!disposed && !busy.value && !loading.value && current?.active) { error.value = null; deactivating.value = current }
  }
  function closeDeactivation(): void { if (!busy.value) deactivating.value = null }

  async function save(): Promise<void> {
    if (disposed || busy.value || loading.value || editingId.value === undefined) return
    const body = couponPayload(draft, original)
    if (!body) { error.value = t('adminCoupons.invalidFields'); return }
    if (!Object.keys(body).length) { close(); return }
    const target = editingId.value
    const ticket = requests.begin()
    busy.value = true
    refreshing.value = false
    error.value = null
    let confirmed = false
    try {
      const result = await featuresApi.saveAdminCoupon(target, body)
      if (!requests.isCurrent(ticket)) return
      items.value = items.value.some(item => item.id === result.id)
        ? items.value.map(item => item.id === result.id ? result : item)
        : [result, ...items.value]
      editingId.value = undefined
      original = null
      confirmed = true
    } catch (caught) {
      if (requests.isCurrent(ticket)) error.value = localizedError(caught, 'adminCoupons.saveFailed')
    } finally {
      if (requests.isCurrent(ticket)) busy.value = false
    }
    if (confirmed && !disposed) await load()
  }

  async function deactivate(): Promise<void> {
    if (disposed || busy.value || loading.value || !deactivating.value) return
    const target = deactivating.value.id
    const ticket = requests.begin()
    busy.value = true
    refreshing.value = false
    error.value = null
    let confirmed = false
    try {
      await featuresApi.deactivateAdminCoupon(target)
      if (!requests.isCurrent(ticket)) return
      items.value = items.value.map(item => item.id === target ? { ...item, active: false } : item)
      deactivating.value = null
      confirmed = true
    } catch (caught) {
      if (requests.isCurrent(ticket)) error.value = localizedError(caught, 'adminCoupons.deactivateFailed')
    } finally {
      if (requests.isCurrent(ticket)) busy.value = false
    }
    if (confirmed && !disposed) await load()
  }

  onMounted(() => { void load(); void loadOptions() })
  onScopeDispose(() => { disposed = true; requests.dispose(); optionRequests.dispose() })
  return { items, loading, refreshing, busy, error, editingId, deactivating, draft, comboItems, squadItems, optionsLoading, optionsError, load, loadOptions, edit, close, requestDeactivation, closeDeactivation, save, deactivate }
}
