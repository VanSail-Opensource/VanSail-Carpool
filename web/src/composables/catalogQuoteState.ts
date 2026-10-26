import { computed, getCurrentInstance, onScopeDispose, readonly, shallowRef } from 'vue'

import { normalizePurchaseQuote } from '@/api/catalog-normalization'
import { ApiError, api } from '@/api/client'
import type { PurchaseQuote } from '@/api/types'

interface QuoteSelection {
  comboId: string
  squadProductIds: string[]
  couponGrantId?: string
}

interface CatalogQuoteOptions {
  selection: () => QuoteSelection | null
  fingerprint: () => string
  onSuccess?: () => void
  onError: (caught: unknown) => void
}

export function useCatalogQuote(options: CatalogQuoteOptions) {
  const quote = shallowRef<PurchaseQuote | null>(null)
  const quoteFingerprint = shallowRef<string | null>(null)
  const quoting = shallowRef(false)
  let requestVersion = 0
  let retryTimer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  if (getCurrentInstance()) onScopeDispose(() => { disposed = true; if (retryTimer) clearTimeout(retryTimer); requestVersion++ })

  const quoteUsable = computed(() => quote.value !== null && quoteFingerprint.value === options.fingerprint())

  function clearQuote(): void {
    if (retryTimer) clearTimeout(retryTimer)
    retryTimer = undefined
    quote.value = null
    quoteFingerprint.value = null
  }

  async function refreshQuote(): Promise<boolean> {
    if (disposed) return false
    if (retryTimer) clearTimeout(retryTimer)
    const selection = options.selection()
    const fingerprint = options.fingerprint()
    const version = ++requestVersion
    if (!selection) {
      clearQuote()
      quoting.value = false
      return false
    }
    quoting.value = true
    try {
      const response = await api.quotePurchase(selection.comboId, selection.squadProductIds, selection.couponGrantId)
      if (version !== requestVersion || options.fingerprint() !== fingerprint) return false
      quote.value = normalizePurchaseQuote(response)
      quoteFingerprint.value = fingerprint
      options.onSuccess?.()
      return true
    } catch (caught) {
      if (version === requestVersion && options.fingerprint() === fingerprint) {
        clearQuote()
        options.onError(caught)
        if (caught instanceof ApiError && caught.code === 'PRICING_DECISION_PENDING') retryTimer = setTimeout(() => { if (!disposed && options.fingerprint() === fingerprint) void refreshQuote() }, 5000)
      }
      return false
    } finally {
      if (version === requestVersion) quoting.value = false
    }
  }

  return { quote: readonly(quote), quoting: readonly(quoting), quoteUsable, clearQuote, refreshQuote }
}
