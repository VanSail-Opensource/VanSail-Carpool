import { describe, expect, it } from 'vitest'
import type { XNodeMetrics } from '@/api/xnode'
import { nodeStatus } from './nodeStatus'

type Metric = XNodeMetrics['nodes'][number]
const current = { connected: true, ready: true, synchronized: true, stale: false } as Metric

describe('native status evidence', () => {
  it('does not describe missing or old observations as disconnected agents', () => {
    expect(nodeStatus().label).toBe('xnode.statusUnavailable')
    expect(nodeStatus({ ...current, connected: false, stale: true }).label).toBe('xnode.stale')
  })
  it('separates connectivity, runtime readiness and applied synchronization', () => {
    expect(nodeStatus({ ...current, connected: false }).label).toBe('xnode.offline')
    expect(nodeStatus({ ...current, ready: false }).label).toBe('xnode.notReady')
    expect(nodeStatus({ ...current, synchronized: false }).label).toBe('xnode.degraded')
    expect(nodeStatus(current).label).toBe('xnode.online')
  })
})
