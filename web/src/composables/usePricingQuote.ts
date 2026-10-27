import { computed, getCurrentScope, onScopeDispose, readonly, shallowRef } from 'vue'
import { ApiError } from '@/api/http'

export interface QuoteSelection { comboId: string; squadProductIds: string[]; couponGrantId?: string }
type QuoteStatus = 'idle' | 'calculating' | 'ready' | 'delayed' | 'stale' | 'unavailable'
interface Options<Q, S> {
 selection: () => S | null
 fingerprint: () => string
 request: (selection: S, signal: AbortSignal) => Promise<Q>
 onSuccess?: () => void
 onError: (caught: unknown) => void
}
export function usePricingQuote<Q extends { quoteValidUntil?: string }, S>(options: Options<Q, S>) {
 const quote = shallowRef<Q | null>(null)
 const quoteFingerprint = shallowRef('')
 const quoting = shallowRef(false)
 const status = shallowRef<QuoteStatus>('idle')
 let controller: AbortController | undefined
 let timer: ReturnType<typeof setTimeout> | undefined
 let expiry: ReturnType<typeof setTimeout> | undefined
 let generation = 0
 let started = 0
 let attempts = 0
 let disposed = false
 const usable = computed(() => status.value === 'ready' && quote.value !== null && quoteFingerprint.value === options.fingerprint())
 function stopTimers() { if (timer) clearTimeout(timer); if (expiry) clearTimeout(expiry); timer = undefined; expiry = undefined }
 function clearQuote() {
  generation++; controller?.abort(); stopTimers()
  quote.value = null; quoteFingerprint.value = ''; quoting.value = false; status.value = 'idle'; started = 0; attempts = 0
 }
 function schedule(fingerprint: string) {
  const remaining = 120_000 - (Date.now() - started)
  if (remaining <= 0) { status.value = 'delayed'; return }
  if (document.hidden) return
  const delay = Math.min(30_000, 5000 * 2 ** attempts++)
  timer = setTimeout(() => { if (!disposed && options.fingerprint() === fingerprint) { if (remaining <= delay) status.value = 'delayed'; else void refreshQuote(true) } }, Math.min(remaining, delay))
 }
 async function refreshQuote(retry = false): Promise<boolean> {
  if (disposed) return false
  const selection = options.selection()
  if (!selection) { clearQuote(); return false }
  controller?.abort(); stopTimers()
  const version = ++generation
  const abort = new AbortController(); controller = abort
  const fingerprint = options.fingerprint()
  if (!retry) { started = Date.now(); attempts = 0 }
  quoting.value = true; status.value = 'calculating'
  try {
   const response = await options.request(selection, abort.signal)
   if (abort.signal.aborted || version !== generation || options.fingerprint() !== fingerprint) return false
   quote.value = response; quoteFingerprint.value = fingerprint; status.value = 'ready'; started = 0
   const until = response.quoteValidUntil ? Date.parse(response.quoteValidUntil) : Number.NaN
   if (Number.isFinite(until)) {
    if (until <= Date.now()) status.value = 'stale'
    else expiry = setTimeout(() => { status.value = 'stale' }, Math.min(2_147_483_647, until - Date.now()))
   }
   options.onSuccess?.()
   return status.value === 'ready'
  } catch (caught) {
   if (!abort.signal.aborted && version === generation && options.fingerprint() === fingerprint) {
    quote.value = null; quoteFingerprint.value = ''
    status.value = caught instanceof ApiError && caught.code === 'PRICING_DECISION_PENDING' ? 'calculating' : 'unavailable'
    options.onError(caught)
    if (status.value === 'calculating') schedule(fingerprint)
   }
   return false
  } finally { if (version === generation) quoting.value = false }
 }
 function visibility() {
  if (document.hidden) { if (timer) clearTimeout(timer); timer = undefined; controller?.abort(); generation++; quoting.value = false }
  else if (status.value === 'calculating') { if (Date.now() - started < 120_000) void refreshQuote(true); else status.value = 'delayed' }
 }
 document.addEventListener('visibilitychange', visibility)
 if (getCurrentScope()) onScopeDispose(() => { disposed = true; clearQuote(); document.removeEventListener('visibilitychange', visibility) })
 return { quote: readonly(quote), quoting: readonly(quoting), quoteUsable: usable, quoteStatus: readonly(status), clearQuote, refreshQuote }
}
