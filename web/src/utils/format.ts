import type { Money, RFC3339 } from '@/api/types'
import { getLocale, t } from '@/i18n'

const unitSymbols: Record<Money['currency'], string> = {
  TXB: 'TXB',
  CNY: '￥',
  USD: '$',
  XTR: 'XTR',
}

export function formatMoney(money: Money): string {
  if (money.display && (money.currency === 'TXB' || money.currency === 'XTR')) return money.display
  const negative = money.minor.startsWith('-')
  const absolute = BigInt(negative ? money.minor.slice(1) : money.minor || '0')
  const scale = money.currency === 'XTR' ? 0 : 2
  const scaleFactor = 10n ** BigInt(scale)
  const whole = absolute / scaleFactor
  const fraction = (absolute % scaleFactor).toString().padStart(scale, '0')
  const numeric = scale > 0 ? `${whole}.${fraction}` : `${whole}`
  const value = negative ? `-${numeric}` : numeric
  if (money.currency === 'TXB' || money.currency === 'XTR') return `${value} ${unitSymbols[money.currency]}`
  return `${negative ? '-' : ''}${unitSymbols[money.currency]}${numeric}`
}

export function formatBytes(raw: string | number): string {
  const normalized = typeof raw === 'number' ? String(Math.trunc(raw)) : raw.trim()
  if (!/^\d+$/.test(normalized)) return '0 B'
  const bytes = BigInt(normalized)
  if (bytes <= 0n) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let index = 0
  let divisor = 1n
  while (index < units.length - 1 && bytes >= divisor * 1024n) {
    divisor *= 1024n
    index += 1
  }
  const whole = bytes / divisor
  const hundredths = (bytes % divisor) * 100n / divisor
  const fraction = hundredths === 0n
    ? '0'
    : hundredths.toString().padStart(2, '0').replace(/0$/, '')
  return `${fraction === '0' ? whole.toString() : `${whole}.${fraction}`} ${units[index]}`
}

export function formatBPS(value: number): string {
  return new Intl.NumberFormat(getLocale(), { maximumFractionDigits: 2 }).format(value / 100)
}

export function trafficBytesFromInput(value: string): string {
  const match = /^\s*(\d+)(?:\.(\d+))?\s*(B|[KMGT]i?B)?\s*$/i.exec(value)
  if (!match) return ''
  const whole = BigInt(match[1])
  const fraction = match[2] ?? ''
  const scale = 10n ** BigInt(fraction.length)
  const unit = (match[3] ?? 'B').toUpperCase()
  const powers: Record<string, bigint> = { B: 1n, KB: 1024n, KIB: 1024n, MB: 1024n ** 2n, MIB: 1024n ** 2n, GB: 1024n ** 3n, GIB: 1024n ** 3n, TB: 1024n ** 4n, TIB: 1024n ** 4n }
  const multiplier = powers[unit]
  if (!multiplier) return ''
  const numerator = (whole * scale + BigInt(fraction || '0')) * multiplier
  if (numerator % scale !== 0n) return ''
  return (numerator / scale).toString()
}

export function formatDate(value?: RFC3339): string {
  if (!value) return t('common.notScheduled')
  return new Intl.DateTimeFormat(getLocale(), {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  }).format(new Date(value))
}

export function formatDateTime(value?: RFC3339): string {
  if (!value) return t('common.notAvailable')
  return new Intl.DateTimeFormat(getLocale(), {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

export function moneyFromTxbInput(value: string): string {
  const normalized = value.trim()
  if (!/^\d+(?:\.\d{0,2})?$/.test(normalized)) return ''
  const [whole, fraction = ''] = normalized.split('.')
  return (BigInt(whole) * 100n + BigInt(fraction.padEnd(2, '0'))).toString()
}

export function signedMoneyFromTxbInput(value: string): string {
  const normalized = value.trim()
  if (!/^[+-]?\d+(?:\.\d{0,2})?$/.test(normalized)) return ''
  const negative = normalized.startsWith('-')
  const unsigned = normalized.replace(/^[+-]/, '')
  const minor = moneyFromTxbInput(unsigned)
  if (minor === '') return ''
  return negative && minor !== '0' ? `-${minor}` : minor
}

export function txbInputFromMinor(value: string): string {
  if (!/^-?\d+$/.test(value)) return ''
  const negative = value.startsWith('-')
  const absolute = BigInt(negative ? value.slice(1) : value)
  const whole = absolute / 100n
  const fraction = (absolute % 100n).toString().padStart(2, '0')
  return `${negative ? '-' : ''}${whole}.${fraction}`
}

export function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join('') || 'TX'
}
