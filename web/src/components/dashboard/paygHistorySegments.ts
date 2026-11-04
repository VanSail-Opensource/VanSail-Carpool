import type { PAYGNodeHistory } from '@/api/payg'
import { statisticsColors } from '@/components/statistics/statisticsFormat'

// Keep exact bytes throughout aggregation. Only drawing shares enter Number,
// bounded to [0,1] at nine decimal places independently of total magnitude.
export function paygHistorySegments(history: PAYGNodeHistory) {
  const rows = history.nodes.map(node => ({ ...node, bytes: BigInt(node.uploadBytes) + BigInt(node.downloadBytes) }))
  const total = rows.reduce((sum, row) => sum + row.bytes, 0n)
  const segments = rows.filter(row => row.bytes > 0n).map((row, i) => {
    const share = total > 0n ? Number(row.bytes * 1_000_000_000n / total) / 1_000_000_000 : 0
    return { ...row, bytes: row.bytes.toString(), interactionId: row.nodeId, share, percentage: share * 100, color: statisticsColors[i % statisticsColors.length]! }
  })
  return { total: total.toString(), segments }
}
