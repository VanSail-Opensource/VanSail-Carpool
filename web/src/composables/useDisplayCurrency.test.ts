import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { DisplayCurrencyPreference } from '@/api/types'

const api = vi.hoisted(() => ({ get: vi.fn(), update: vi.fn() }))
vi.mock('@/api/displayCurrency', () => ({ displayCurrencyApi: api }))

import { useDisplayCurrency } from './useDisplayCurrency'

function preference(currency: DisplayCurrencyPreference['currency']): DisplayCurrencyPreference {
  return { currency, rates: { cnyTxbPerUnit: '10', usdTxbPerUnit: '70' } }
}

function deferred() {
  let resolve!: (value: DisplayCurrencyPreference) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<DisplayCurrencyPreference>((accept, fail) => { resolve = accept; reject = fail })
  return { promise, resolve, reject }
}

beforeEach(() => {
  vi.resetAllMocks()
  useDisplayCurrency().hydrate('TXB')
})

describe('display currency request ownership', () => {
  it('keeps the completed selection when an earlier read finishes later', async () => {
    const read = deferred()
    api.get.mockReturnValueOnce(read.promise)
    api.update.mockResolvedValueOnce(preference('USD'))
    const state = useDisplayCurrency()
    const loading = state.refresh()

    expect(await state.select('USD')).toBe(true)
    read.resolve(preference('CNY'))
    await loading

    expect(state.currency.value).toBe('USD')
    expect(state.rates.value).toEqual(preference('USD').rates)
    expect(state.loading.value).toBe(false)
    expect(state.saving.value).toBe(false)
    expect(state.error.value).toBeNull()
  })

  it('allows a new owner to refresh while an old read is pending and ignores its failure', async () => {
    const previous = deferred()
    const current = deferred()
    api.get.mockReturnValueOnce(previous.promise).mockReturnValueOnce(current.promise)
    const state = useDisplayCurrency()
    const oldRead = state.refresh()
    state.hydrate('USD')
    const newRead = state.refresh()

    previous.reject(new Error('previous owner offline'))
    await oldRead
    expect(state.loading.value).toBe(true)
    expect(state.currency.value).toBe('USD')
    expect(state.error.value).toBeNull()

    current.resolve(preference('USD'))
    await newRead
    expect(state.loading.value).toBe(false)
    expect(state.rates.value).toEqual(preference('USD').rates)
  })

  it('does not let a previous owner save complete or unlock the next owner save', async () => {
    const previous = deferred()
    const current = deferred()
    api.update.mockReturnValueOnce(previous.promise).mockReturnValueOnce(current.promise)
    const state = useDisplayCurrency()
    const oldSave = state.select('CNY')
    state.hydrate('USD')
    const newSave = state.select('CNY')

    previous.resolve(preference('CNY'))
    expect(await oldSave).toBe(false)
    expect(state.saving.value).toBe(true)
    expect(state.currency.value).toBe('USD')

    current.resolve(preference('CNY'))
    expect(await newSave).toBe(true)
    expect(state.saving.value).toBe(false)
    expect(state.currency.value).toBe('CNY')
  })

  it('clears both currency and rates on logout and discards the pending reply', async () => {
    const read = deferred()
    api.get.mockResolvedValueOnce(preference('USD')).mockReturnValueOnce(read.promise)
    const state = useDisplayCurrency()
    await state.refresh()
    const pending = state.refresh()
    state.hydrate(undefined)
    read.resolve(preference('USD'))
    await pending

    expect(state.currency.value).toBe('TXB')
    expect(state.rates.value).toEqual({ cnyTxbPerUnit: null, usdTxbPerUnit: null })
    expect(state.loading.value).toBe(false)
    expect(state.error.value).toBeNull()
  })
})
