import type { PAYGNodeHistory } from '@/api/payg'
import { statisticsColors } from '@/components/statistics/statisticsFormat'

// Keep exact bytes or minor-unit charges throughout aggregation. Only drawing shares enter Number,
// bounded to [0,1] at nine decimal places independently of total magnitude.
export type PAYGHistoryMetric = 'traffic' | 'cost'
export function paygHistorySegments(history: PAYGNodeHistory, metric: PAYGHistoryMetric = 'traffic') {
  const rows = history.nodes.map(node => ({ ...node, bytes: metric === 'cost' ? BigInt(node.spentMinor ?? '0') : BigInt(node.uploadBytes) + BigInt(node.downloadBytes) }))
  const total = rows.reduce((sum, row) => sum + row.bytes, 0n)
  const nonEmptyRows = rows.filter(row => row.bytes > 0n)
  const visibleRows = nonEmptyRows.filter(row => row.bytes * 100n >= total)
  const otherRows = nonEmptyRows.filter(row => row.bytes * 100n < total)
  const segments = visibleRows.map((row, i) => {
    const share = total > 0n ? Number(row.bytes * 1_000_000_000n / total) / 1_000_000_000 : 0
    return { ...row, bytes: row.bytes.toString(), interactionId: row.nodeId, share, percentage: share * 100, color: statisticsColors[i % statisticsColors.length]!, isOther: false }
  })
  if (otherRows.length) {
    const bytes = otherRows.reduce((sum, row) => sum + row.bytes, 0n)
    const share = Number(bytes * 1_000_000_000n / total) / 1_000_000_000
    const nodeIds = new Set(rows.map(row => row.nodeId))
    let nodeId = '__payg_others__'
    while (nodeIds.has(nodeId)) nodeId += '_'
    segments.push({
      ...otherRows[0]!,
      nodeId,
      name: '',
      countryCode: '',
      bytes: bytes.toString(),
      interactionId: nodeId,
      share,
      percentage: share * 100,
      color: statisticsColors[segments.length % statisticsColors.length]!,
      isOther: true,
    })
  }
  return { total: total.toString(), segments }
}
