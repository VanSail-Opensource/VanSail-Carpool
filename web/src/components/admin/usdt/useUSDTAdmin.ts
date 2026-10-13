import { onMounted, onScopeDispose, shallowRef } from 'vue'
import { usdtApi, type USDTConfig, type USDTNetworkID, type USDTReviewPage, type USDTStatus } from '@/api/usdt'
import { localizedError, t } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'
import { notifyHaptic } from '@/utils/telegram'

export function useUSDTAdmin() {
  const draft = shallowRef<USDTConfig | null>(null)
  const status = shallowRef<USDTStatus | null>(null)
  const review = shallowRef<USDTReviewPage>({ items: [], nextCursor: '' })
  const loading = shallowRef(true)
  const busy = shallowRef(false)
  const checking = shallowRef<USDTNetworkID | null>(null)
  const resolving = shallowRef<string | null>(null)
  const error = shallowRef<string | null>(null)
  const saved = shallowRef(false)
  const probe = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  let active = true
  let timer: ReturnType<typeof setTimeout> | undefined

  async function load(): Promise<void> {
    const token = latest.begin()
    loading.value = true
    error.value = null
    const results = await Promise.allSettled([usdtApi.config(), usdtApi.status(), usdtApi.review()])
    if (!active || !latest.isCurrent(token)) return
    const [configuration, currentStatus, transfers] = results
    if (configuration.status === 'fulfilled') draft.value = configuration.value
    if (currentStatus.status === 'fulfilled') status.value = currentStatus.value
    if (transfers.status === 'fulfilled') review.value = transfers.value
    for (const result of results) if (result.status === 'rejected') error.value = localizedError(result.reason, 'errors.adminLoad')
    loading.value = false
  }
  async function refresh(): Promise<void> {
    try { const value = await usdtApi.status(); if (active) status.value = value }
    catch (caught) { if (active) error.value = localizedError(caught, 'errors.adminLoad') }
  }
  function schedule(): void {
    timer = setTimeout(async () => {
      if (document.visibilityState === 'visible') await refresh()
      if (active) schedule()
    }, 15000)
  }
  async function save(): Promise<void> {
    if (!draft.value || busy.value) return
    busy.value = true; error.value = null; saved.value = false
    try {
      const value = await usdtApi.save(draft.value)
      if (active) { draft.value = value; saved.value = true; await refresh(); notifyHaptic('success') }
    } catch (caught) { if (active) error.value = localizedError(caught, 'errors.adminAction') }
    finally { if (active) busy.value = false }
  }
  async function check(network: USDTNetworkID): Promise<void> {
    if (checking.value) return
    checking.value = network; probe.value = null
    try { const result = await usdtApi.check(network); if (active) probe.value = result.success ? t('adminUsdt.checkOK') : t(`errors.api.${result.errorCode}`) }
    catch (caught) { if (active) probe.value = localizedError(caught, 'errors.adminAction') }
    finally { if (active) checking.value = null }
  }
  async function loadReview(more = false): Promise<void> {
    try {
      const value = await usdtApi.review(more ? review.value.nextCursor : undefined)
      if (active) review.value = { ...value, items: more ? [...review.value.items, ...value.items] : value.items }
    } catch (caught) { if (active) error.value = localizedError(caught, 'errors.adminLoad') }
  }
  async function resolve(id: string, orderId: string, reason: string): Promise<void> {
    if (resolving.value) return
    resolving.value = id; error.value = null
    try { await usdtApi.resolve(id, orderId, reason); if (active) { await loadReview(); notifyHaptic('success') } }
    catch (caught) { if (active) error.value = localizedError(caught, 'errors.adminAction') }
    finally { if (active) resolving.value = null }
  }
  onMounted(() => { void load(); schedule() })
  onScopeDispose(() => { active = false; latest.invalidate(); if (timer !== undefined) clearTimeout(timer) })
  return { draft, status, review, loading, busy, checking, resolving, error, saved, probe, load, refresh, save, check, loadReview, resolve }
}
