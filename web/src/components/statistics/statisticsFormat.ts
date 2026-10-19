import type { NamedShare } from '@/api/types'
import { getLocale } from '@/i18n'
import { formatBytes } from '@/utils/format'

export const statisticsColors = [
  '#a6d9bb', '#d8c18f', '#8fb5d8', '#e2a29e',
  '#b8a7d9', '#78c7bf', '#d9a37d', '#aebc86',
] as const

export interface StatisticSegment extends NamedShare {
  color: string
  percentage: number
}

function finite(value: number): number {
  return Number.isFinite(value) && value > 0 ? value : 0
}

export function chartSegments(items: readonly NamedShare[], share = 100): StatisticSegment[] {
  const total = items.reduce((sum, item) => sum + finite(item.value), 0)
  if (total <= 0) return []
  const scale = Number.isFinite(share) && share > 0 ? share : 100
  return items
    .filter((item) => finite(item.value) > 0)
    .map((item, index) => ({
      ...item,
      color: statisticsColors[index % statisticsColors.length],
      percentage: finite(item.value) * scale / total,
    }))
}

export function formatStatisticNumber(value: number, maximumFractionDigits = 0): string {
  return new Intl.NumberFormat(getLocale(), { maximumFractionDigits }).format(Number.isFinite(value) ? value : 0)
}

export function formatStatisticPercent(value: number): string {
  return `${formatStatisticNumber(value, 1)}%`
}

export function formatSignedStatistic(value: number): string {
  const normalized = Number.isFinite(value) ? value : 0
  return `${normalized > 0 ? '+' : ''}${formatStatisticNumber(normalized)}`
}

export function formatShortStatisticDate(value: string): string {
  const date = new Date(`${value}T00:00:00.000Z`)
  if (!Number.isFinite(date.valueOf())) return value
  return new Intl.DateTimeFormat(getLocale(), { weekday: 'short', timeZone: 'UTC' }).format(date)
}

export function safeStatisticBytes(value: string): bigint {
  try {
    return /^\d+$/.test(value) ? BigInt(value) : 0n
  } catch {
    return 0n
  }
}

export function sumStatisticBytes(values: readonly string[]): bigint {
  return values.reduce((sum, value) => sum + safeStatisticBytes(value), 0n)
}

export function formatStatisticBytes(value: bigint): string {
  return formatBytes(value.toString())
}

export function shareTotal(items: readonly NamedShare[]): number {
  return items.reduce((sum, item) => sum + finite(item.value), 0)
}
