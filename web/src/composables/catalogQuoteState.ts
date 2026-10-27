import { normalizePurchaseQuote } from '@/api/catalog-normalization'
import { api } from '@/api/client'
import { usePricingQuote, type QuoteSelection } from './usePricingQuote'
interface CatalogQuoteOptions {
 selection: () => QuoteSelection | null
 fingerprint: () => string
 onSuccess?: () => void
 onError: (caught: unknown) => void
}
export function useCatalogQuote(options: CatalogQuoteOptions) {
 return usePricingQuote({
  ...options,
  request: async (selection: QuoteSelection, signal: AbortSignal) => normalizePurchaseQuote(await api.quotePurchase(selection.comboId, selection.squadProductIds, selection.couponGrantId, signal)),
 })
}
