import type { NamedShare } from '@/api/types'
import { getLocale } from '@/i18n'

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

export function shareTotal(items: readonly NamedShare[]): number {
  return items.reduce((sum, item) => sum + finite(item.value), 0)
}
