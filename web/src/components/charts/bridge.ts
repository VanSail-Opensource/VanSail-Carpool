import type { ChartDatum } from './index'

// The bridge is presentation-only; original rows remain authoritative for inspection.
export function bridgeSeries(rows: readonly ChartDatum[], fromKey: string, toKey: string): ChartDatum[] {
  const from = rows.filter(row => Number.isFinite(row[fromKey])).at(-1)
  const to = rows.find(row => Number.isFinite(row[toKey]) && (!from || row.x > from.x))
  if (!from || !to) return []
  return [{ ...from, bridgeValue: Number(from[fromKey]) }, { ...to, bridgeValue: Number(to[toKey]) }]
}
