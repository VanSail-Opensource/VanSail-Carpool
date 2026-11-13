import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { z } from 'zod'
import { useRoute, useRouter } from 'vue-router'
import { ipLookupApi, type IPLookupCheck, type IPLookupQuote, type IPLookupState, type IPLookupReport } from '@/api/ipLookup'
import { ApiError } from '@/api/http'
import { useSessionStore } from '@/stores/session'
import { createUuid } from '@/utils/browserCompatibility'
import { useSharedIPProgress } from './useSharedIPProgress'

const terminal = new Set(['succeeded', 'partial', 'failed', 'compensated', 'pending_review'])
const ipSchema = z.union([z.ipv4(), z.ipv6()])
type CacheMatch = IPLookupQuote['cacheMatch']

export function useIPLookup() {
  const session = useSessionStore()
  const route = useRoute()
  const router = useRouter()
  const ip = shallowRef('')
  const state = shallowRef<IPLookupState | null>(null)
  const quote = shallowRef<IPLookupQuote | null>(null)
  const refreshQuote = shallowRef<IPLookupQuote | null>(null)
  const previousReport = shallowRef<IPLookupReport | null>(null)
  const previousCacheMatch = shallowRef<CacheMatch>('none')
  const check = shallowRef<IPLookupCheck | null>(null)
  const loading = shallowRef(true)
  const busy = shallowRef(false)
  const errorCode = shallowRef('')
  const validIP = computed(() => ipSchema.safeParse(ip.value.trim()).success)
  const { activeRun, checking, reloadProgress, progressUnavailable } = useSharedIPProgress(() => state.value?.enabled && validIP.value ? ip.value.trim() : '', () => {
    if (activeCheck) void poll(activeCheck.id, activeCheck.owner, activeCheck.scope)
    else if (!busy.value) { void load(); void updateQuote() }
  })
  const displayBusy = computed(() => busy.value || checking.value)
  let disposed = false
  let revision = 0
  let sessionRevision = 0
  let scopeRevision = 0
  let stateRevision = 0
  let pollRevision = 0
  let quoteTimer: ReturnType<typeof setTimeout> | undefined
  let pollTimer: ReturnType<typeof setTimeout> | undefined
  const pendingSubmissions = new Map<string, { quote: IPLookupQuote; key: string }>()
  const acceptedChecks = new Map<string, string>()
  let activeCheck: { id: string; owner: ReturnType<typeof ownerSnapshot>; scope: number } | null = null
  const ownerSnapshot = () => ({ id: session.user?.id, version: sessionRevision })
  const isCurrent = (owner: ReturnType<typeof ownerSnapshot>) => !disposed && owner.id === session.user?.id && owner.version === sessionRevision
  const report = computed(() => check.value?.report ?? quote.value?.cachedReport ?? previousReport.value)
  const requestedIP = computed(() => check.value?.requestedIP ?? quote.value?.ip ?? ip.value)
  const cacheMatch = computed(() => check.value?.report ? check.value.cacheMatch : quote.value?.cacheMatch ?? previousCacheMatch.value)
  const displayCheck = computed(() => check.value?.report ? check.value : null)
  const setError = (error: unknown) => { errorCode.value = error instanceof ApiError ? error.code : 'IP_LOOKUP_FAILED' }

  async function load(): Promise<void> {
    const owner = ownerSnapshot(), ticket = ++stateRevision
    const current = () => isCurrent(owner) && ticket === stateRevision
    try { const value = await ipLookupApi.state(); if (current()) state.value = value }
    catch (error) { if (current()) setError(error) }
    finally { if (current()) loading.value = false }
  }

  async function updateQuote(): Promise<void> {
    const version = ++revision
    const owner = ownerSnapshot()
    quote.value = null
    refreshQuote.value = null
    if (!validIP.value || !state.value?.enabled || busy.value) return
    try {
      const value = await ipLookupApi.quote(ip.value.trim(), false)
      if (version !== revision || !isCurrent(owner)) return
      quote.value = value
      if (value.cachedReport) {
        const priced = await ipLookupApi.quote(value.ip, true)
        if (version === revision && isCurrent(owner)) refreshQuote.value = priced
      }
    } catch (error) { if (version === revision && isCurrent(owner)) setError(error) }
  }

  async function poll(id: string, owner: ReturnType<typeof ownerSnapshot>, scope = scopeRevision): Promise<void> {
    if (!isCurrent(owner) || scope !== scopeRevision) return
    activeCheck = { id, owner, scope }
    const viewIP = ip.value.trim(), ticket = ++pollRevision
    const current = () => isCurrent(owner) && scope === scopeRevision && ticket === pollRevision && (!viewIP || ip.value.trim() === viewIP)
    clearTimeout(pollTimer)
    if (document.hidden) return
    const schedule = () => { pollTimer = setTimeout(() => void poll(id, owner, scope), 5000) }
    try {
      const value = await ipLookupApi.check(id)
      if (!current()) return
      check.value = value
      const address = value.requestedIP || value.report?.ip || viewIP
      if (address) acceptedChecks.set(address, id)
      if (!ip.value) ip.value = address
      if (terminal.has(value.operation.status)) {
        activeCheck = null
        busy.value = false
        errorCode.value = value.operation.errorCode ?? ''
        await load()
        if (current()) await updateQuote()
      } else schedule()
    } catch (error) {
      if (!current()) return
      setError(error)
      if (error instanceof ApiError && error.status < 500 && error.status !== 429) { activeCheck = null; busy.value = false; return }
      schedule()
    }
  }

  async function submit(refresh = false): Promise<void> {
    if (busy.value || checking.value) return
    const address = ip.value.trim()
    const retained = pendingSubmissions.get(address)
    const selected = retained?.quote ?? (refresh ? refreshQuote.value : quote.value)
    if (!selected || (!refresh && selected.cachedReport) || !validIP.value) return
    if (retained && retained.quote.refresh !== refresh) return
    if (!retained && selected.expiresAt <= Math.floor(Date.now() / 1000)) { await updateQuote(); return }
    if (refresh) { previousReport.value = report.value; previousCacheMatch.value = cacheMatch.value }
    busy.value = true
    errorCode.value = ''
    const owner = ownerSnapshot(), scope = scopeRevision
    const current = () => isCurrent(owner) && scope === scopeRevision && address === ip.value.trim()
    const attempt = retained ?? { quote: selected, key: createUuid() }
    pendingSubmissions.set(address, attempt)
    try {
      const operation = await ipLookupApi.submit(attempt.quote, attempt.key)
      if (!isCurrent(owner)) return
      // Acceptance is durable even if the member has already entered another IP.
      acceptedChecks.set(address, operation.id)
      if (pendingSubmissions.get(address) === attempt) pendingSubmissions.delete(address)
      if (!current()) return
      void reloadProgress()
      void router.replace({ query: { ...route.query, check: operation.id } }).catch(() => undefined)
      await poll(operation.id, owner, scope)
    } catch (error) {
      if (!isCurrent(owner)) return
      if (error instanceof ApiError && error.status < 500 && error.status !== 429 && pendingSubmissions.get(address) === attempt) pendingSubmissions.delete(address)
      if (!current()) return
      busy.value = false
      setError(error)
      if (!pendingSubmissions.has(address)) { await load(); if (current()) await updateQuote() }
    }
  }

  watch(ip, () => {
    if (check.value?.requestedIP === ip.value.trim()) return
    revision++
    scopeRevision++
    pollRevision++
    activeCheck = null
    quote.value = null
    refreshQuote.value = null
    previousReport.value = null
    previousCacheMatch.value = 'none'
    errorCode.value = ''
    clearTimeout(quoteTimer)
    clearTimeout(pollTimer)
    busy.value = false
    check.value = null
    const accepted = acceptedChecks.get(ip.value.trim())
    if (accepted) { busy.value = true; void poll(accepted, ownerSnapshot()) }
    else quoteTimer = setTimeout(() => void updateQuote(), 350)
  }, { flush: 'sync' })
  watch(() => session.user?.id, () => {
    revision++
    sessionRevision++
    scopeRevision++
    pollRevision++
    activeCheck = null
    clearTimeout(quoteTimer)
    clearTimeout(pollTimer)
    state.value = quote.value = check.value = null
    refreshQuote.value = null
    previousReport.value = null
    previousCacheMatch.value = 'none'
    ip.value = ''
    errorCode.value = ''
    pendingSubmissions.clear()
    acceptedChecks.clear()
    busy.value = false
    loading.value = true
    void load()
    if (session.user?.id && typeof route.query.check === 'string') {
      busy.value = true
      void poll(route.query.check, ownerSnapshot())
    }
  }, { immediate: true, flush: 'sync' })
  const onVisibility = () => {
    clearTimeout(pollTimer)
    if (!document.hidden && activeCheck) void poll(activeCheck.id, activeCheck.owner, activeCheck.scope)
  }
  document.addEventListener('visibilitychange', onVisibility)
  onScopeDispose(() => { disposed = true; revision++; pollRevision++; activeCheck = null; pendingSubmissions.clear(); acceptedChecks.clear(); clearTimeout(quoteTimer); clearTimeout(pollTimer); document.removeEventListener('visibilitychange', onVisibility) })
  return { ip, state, quote, refreshQuote, check, displayCheck, report, requestedIP, cacheMatch, loading, busy: displayBusy, activeRun, progressUnavailable, errorCode, validIP, submit, load }
}
