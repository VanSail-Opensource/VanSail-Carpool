import type { XNodeProfileMetrics } from '@/api/adminOperations'
import type { ChartDatum } from '@/components/charts'

export function paygHistorySeries(history: XNodeProfileMetrics['paygHistory'], nodeId = ''): ChartDatum[] {
  const start = Math.floor(Date.parse(history.from) / 3600000) * 3600000
  const end = Math.floor(Date.parse(history.to) / 3600000) * 3600000
  if (!Number.isFinite(start) || !Number.isFinite(end)) return []
  const hours = new Map<number, { upload: bigint; download: bigint }>()
  for (const row of history.hours) {
    if (nodeId && row.nodeId !== nodeId) continue
    const at = Date.parse(row.hour)
    const value = hours.get(at) ?? { upload: 0n, download: 0n }
    value.upload += BigInt(row.uploadBytes); value.download += BigInt(row.downloadBytes)
    hours.set(at, value)
  }
  const rows: ChartDatum[] = []
  for (let at = start; at <= end && rows.length < 73; at += 3600000) {
    const value = hours.get(at) ?? { upload: 0n, download: 0n }
    rows.push({ x: at, label: '', upload: Number(value.upload), download: Number(value.download), uploadExact: value.upload.toString(), downloadExact: value.download.toString() })
  }
  return rows
}
