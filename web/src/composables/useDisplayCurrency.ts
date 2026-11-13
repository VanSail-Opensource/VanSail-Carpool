import { computed, readonly, shallowRef } from 'vue'

import { displayCurrencyApi } from '@/api/displayCurrency'
import type { DisplayCurrencyPreference } from '@/api/types'
import { t } from '@/i18n'
import { displayCurrencyState, resetDisplayCurrency, setDisplayCurrency } from '@/utils/displayCurrency'

const saving = shallowRef(false)
const loading = shallowRef(false)
const error = shallowRef<string | null>(null)
let revision = 0

export function useDisplayCurrency() {
  const currencies = computed<DisplayCurrencyPreference['currency'][]>(() => ['TXB', 'CNY', 'USD'])

  async function refresh(): Promise<void> {
    if (loading.value || saving.value) return
    const current = ++revision
    loading.value = true
    try {
      const preference = await displayCurrencyApi.get()
      if (current !== revision) return
      setDisplayCurrency(preference)
      error.value = null
    } catch {
      if (current !== revision) return
      if (displayCurrencyState.currency.value !== 'TXB') error.value = t('app.currencyRateUnavailable')
    } finally {
      if (current === revision) loading.value = false
    }
  }

  async function select(next: DisplayCurrencyPreference['currency']): Promise<boolean> {
    if (saving.value || next === displayCurrencyState.currency.value) return false
    const current = ++revision
    loading.value = false
    saving.value = true
    try {
      const preference = await displayCurrencyApi.update(next)
      if (current !== revision) return false
      setDisplayCurrency(preference)
      error.value = null
      return true
    } catch {
      if (current !== revision) return false
      error.value = t('app.currencyRateUnavailable')
      return false
    } finally {
      if (current === revision) saving.value = false
    }
  }

  function hydrate(currency: DisplayCurrencyPreference['currency'] | undefined): void {
    revision++
    loading.value = false
    saving.value = false
    resetDisplayCurrency(currency ?? 'TXB')
    error.value = null
  }

  return {
    currency: displayCurrencyState.currency,
    rates: displayCurrencyState.rates,
    currencies,
    saving: readonly(saving),
    loading: readonly(loading),
    error: readonly(error),
    refresh,
    select,
    hydrate,
  }
}
