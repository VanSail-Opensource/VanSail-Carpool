import { onMounted, onScopeDispose, shallowRef } from 'vue'
import { usdtApi, type USDTConfig, type USDTNetworkID, type USDTReviewPage, type USDTStatus } from '@/api/usdt'
import { localizedError, t } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useUSDTAdmin() {
  const draft = shallowRef<USDTConfig | null>(null)
  const status = shallowRef<USDTStatus | null>(null)
  const review = shallowRef<USDTReviewPage>({ items: [], nextCursor: '' })
  const loading = shallowRef(true)
  const busy = shallowRef(false)
  const checking = shallowRef<string | null>(null)
  const resolving = shallowRef<string | null>(null)
  const error = shallowRef<string | null>(null)
  const saved = shallowRef(false)
  const probe = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  const statusLatest = createLatestRequest()
  const reviewLatest = createLatestRequest()
  const loaded = shallowRef(false)
  let loadController: AbortController | undefined
  let statusController: AbortController | undefined
  let reviewController: AbortController | undefined
  let active = true
  let timer: ReturnType<typeof setTimeout> | undefined

  async function load(): Promise<void> {
    loadController?.abort(); statusController?.abort(); reviewController?.abort()
    statusLatest.invalidate(); reviewLatest.invalidate()
    loadController = new AbortController()
    const token = latest.begin()
    loaded.value = false; draft.value = null
    loading.value = true
    error.value = null
    const results = await Promise.allSettled([usdtApi.config(loadController.signal), usdtApi.status(loadController.signal), usdtApi.review(undefined, loadController.signal)])
    if (!active || !latest.isCurrent(token)) return
    const [configuration, currentStatus, transfers] = results
    if (configuration.status === 'fulfilled') draft.value = configuration.value
    if (currentStatus.status === 'fulfilled') status.value = currentStatus.value
    if (transfers.status === 'fulfilled') review.value = transfers.value
    for (const result of results) if (result.status === 'rejected') error.value = localizedError(result.reason, 'errors.adminLoad')
    loaded.value = results.every(result => result.status === 'fulfilled')
    loading.value = false
  }
  async function refresh(): Promise<void> {
    statusController?.abort(); statusController = new AbortController()
    const generation = statusLatest.begin()
    try { const value = await usdtApi.status(statusController.signal); if (active && statusLatest.isCurrent(generation)) status.value = value }
    catch (caught) { if (active && statusLatest.isCurrent(generation)) error.value = localizedError(caught, 'errors.adminLoad') }
  }
  function schedule(): void {
    timer = setTimeout(async () => {
      if (document.visibilityState === 'visible') await refresh()
      if (active) schedule()
    }, 15000)
  }
  async function save(): Promise<void> {
    if (!draft.value || busy.value || loading.value || !loaded.value) return
    const generation = latest.begin()
    busy.value = true; error.value = null; saved.value = false
    try {
      const value = await usdtApi.save(draft.value)
      if (active && latest.isCurrent(generation)) { draft.value = value; saved.value = true; await refresh(); }
    } catch (caught) { if (active) error.value = localizedError(caught, 'errors.adminAction') }
    finally { if (active) busy.value = false }
  }
  async function check(network: USDTNetworkID, rail?: string): Promise<void> {
    if (checking.value) return
    checking.value = rail ?? network; probe.value = null
    try { const result = await usdtApi.check(network, rail); if (active) probe.value = result.success ? t('adminUsdt.checkOK') : t(`errors.api.${result.errorCode}`) }
    catch (caught) { if (active) probe.value = localizedError(caught, 'errors.adminAction') }
    finally { if (active) checking.value = null }
  }
  async function loadReview(more = false): Promise<void> {
    reviewController?.abort(); reviewController = new AbortController()
    const generation = reviewLatest.begin()
    try {
      const value = await usdtApi.review(more ? review.value.nextCursor : undefined, reviewController.signal)
      if (active && reviewLatest.isCurrent(generation)) review.value = { ...value, items: more ? [...review.value.items, ...value.items] : value.items }
    } catch (caught) { if (active && reviewLatest.isCurrent(generation)) error.value = localizedError(caught, 'errors.adminLoad') }
  }
  async function dismiss(id: string, reason: string): Promise<void> {
    if (resolving.value) return
    resolving.value = id; error.value = null
    try { await usdtApi.dismiss(id, reason); if (active) { await loadReview(); } }
    catch (caught) { if (active) error.value = localizedError(caught, 'errors.adminAction') }
    finally { if (active) resolving.value = null }
  }
  onMounted(() => { void load(); schedule() })
  onScopeDispose(() => { active = false; latest.dispose(); statusLatest.dispose(); reviewLatest.dispose(); loadController?.abort(); statusController?.abort(); reviewController?.abort(); if (timer !== undefined) clearTimeout(timer) })
  return { draft, status, review, loading, loaded, busy, checking, resolving, error, saved, probe, load, refresh, save, check, loadReview, dismiss }
}
