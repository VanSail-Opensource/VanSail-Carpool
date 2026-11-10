import { readonly, shallowRef } from 'vue'

import type { DisplayCurrencyPreference, Money } from '@/api/types'
import { formatMoney, moneyFromTxbInput, txbInputFromMinor } from './format'

type DisplayCurrency = DisplayCurrencyPreference['currency']

const currency = shallowRef<DisplayCurrency>('TXB')
const rates = shallowRef<DisplayCurrencyPreference['rates']>({ cnyTxbPerUnit: null, usdTxbPerUnit: null })

export const displayCurrencyState = {
  currency: readonly(currency),
  rates: readonly(rates),
}

export function setDisplayCurrency(preference: DisplayCurrencyPreference): void {
  currency.value = preference.currency
  rates.value = preference.rates
}

export function resetDisplayCurrency(value: DisplayCurrency = 'TXB'): void {
  currency.value = value
  rates.value = { cnyTxbPerUnit: null, usdTxbPerUnit: null }
}

export function formatMemberMoney(money: Money): string {
  const target = currency.value
  if (target === 'TXB' || money.currency !== 'TXB') return formatMoney(money)
  const rate = target === 'CNY' ? rates.value.cnyTxbPerUnit : rates.value.usdTxbPerUnit
  const converted = rate ? convertTXBMoney(money.minor, target, rate) : null
  if (!converted || !isNonzeroMinor(money.minor) || !isZeroConvertedMoney(converted)) return converted ?? formatMoney(money)

  const cnyFallback = target === 'USD' && rates.value.cnyTxbPerUnit
    ? convertTXBMoney(money.minor, 'CNY', rates.value.cnyTxbPerUnit)
    : null
  return cnyFallback && !isZeroConvertedMoney(cnyFallback) ? cnyFallback : formatMoney(money)
}

export function formatCatalogMoney(money: Money): string {
  const native = formatMoney(money)
  const converted = formatMemberMoney(money)
  return converted === native ? native : `${native} (${converted})`
}

export interface CatalogMoneyLines {
  primary: string
  approximation: string | null
}

export function formatCatalogMoneyLines(money: Money): CatalogMoneyLines {
  const primary = formatMoney(money)
  const converted = formatMemberMoney(money)
  return {
    primary,
    approximation: converted === primary ? null : `≈${converted}`,
  }
}

export function displayInputFromTXBMinor(minor: string): string {
  const target = currency.value
  if (target === 'TXB') return txbInputFromMinor(minor)
  const rate = target === 'CNY' ? rates.value.cnyTxbPerUnit : rates.value.usdTxbPerUnit
  const converted = rate ? convertTXBMoney(minor, target, rate) : null
  return converted?.replace(/^(-?)(?:￥|\$)/, '$1') ?? txbInputFromMinor(minor)
}

// A display currency can be persisted while its rate is later removed. Amount
// inputs then keep working in TXB instead of pricing through a rate the member
// no longer has; the app already renders TXB amounts in that state.
export function displayInputCurrency(): DisplayCurrency {
  const target = currency.value
  if (target === 'TXB') return 'TXB'
  const rate = target === 'CNY' ? rates.value.cnyTxbPerUnit : rates.value.usdTxbPerUnit
  return rate && parseRate(rate) ? target : 'TXB'
}

export function displayInputMinimumFromTXBMinor(minor: string): string {
  const target = currency.value
  if (target === 'TXB') return txbInputFromMinor(minor)
  const rate = target === 'CNY' ? rates.value.cnyTxbPerUnit : rates.value.usdTxbPerUnit
  const parsedRate = rate ? parseRate(rate) : null
  if (!parsedRate || !/^\d+$/.test(minor)) return displayInputFromTXBMinor(minor)
  const source = BigInt(minor)
  const numerator = source * (10n ** BigInt(parsedRate.scale))
  const targetMinor = (numerator + parsedRate.coefficient - 1n) / parsedRate.coefficient
  return `${targetMinor / 100n}.${(targetMinor % 100n).toString().padStart(2, '0')}`
}

export function displayInputToTXBMinor(value: string): string {
  const target = currency.value
  const numeric = Number(value)
  const inputMinor = Number.isFinite(numeric) && numeric >= 0 ? moneyFromTxbInput(numeric.toFixed(2)) : ''
  if (inputMinor === '' || target === 'TXB') return inputMinor
  const rate = target === 'CNY' ? rates.value.cnyTxbPerUnit : rates.value.usdTxbPerUnit
  const parsedRate = rate ? parseRate(rate) : null
  if (!parsedRate) return ''
  return (BigInt(inputMinor) * parsedRate.coefficient / (10n ** BigInt(parsedRate.scale))).toString()
}

export function convertTXBMoney(minor: string, target: Exclude<DisplayCurrency, 'TXB'>, rate: string): string | null {
  const parsedRate = parseRate(rate)
  if (!parsedRate || !/^-?\d+$/.test(minor)) return null
  const source = BigInt(minor)
  const negative = source < 0n
  const absolute = negative ? -source : source
  const targetMinor = absolute * (10n ** BigInt(parsedRate.scale)) / parsedRate.coefficient
  const whole = targetMinor / 100n
  const fraction = (targetMinor % 100n).toString().padStart(2, '0')
  const symbol = target === 'CNY' ? '￥' : '$'
  return `${negative && targetMinor > 0n ? '-' : ''}${symbol}${whole}.${fraction}`
}

function isNonzeroMinor(value: string): boolean {
  return /^-?\d+$/.test(value) && BigInt(value) !== 0n
}

function isZeroConvertedMoney(value: string): boolean {
  return /^-?[￥$]0\.00$/.test(value)
}

function parseRate(value: string): { coefficient: bigint; scale: number } | null {
  const normalized = value.trim()
  if (!/^\d+(?:\.\d+)?$/.test(normalized)) return null
  const [whole, fraction = ''] = normalized.split('.')
  const coefficient = BigInt(`${whole}${fraction}`)
  return coefficient > 0n ? { coefficient, scale: fraction.length } : null
}
