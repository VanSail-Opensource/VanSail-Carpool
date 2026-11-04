import { expect, it } from 'vitest'
import type { XNodeProfileMetrics } from '@/api/adminOperations'
import { paygHistorySeries } from './userTrafficHistory'

const history: XNodeProfileMetrics['paygHistory'] = {
  from: '2026-11-01T03:20:00Z', to: '2026-11-04T03:20:59Z', truncated: false, nodes: [],
  hours: [
    { nodeId: 'current', hour: '2026-11-04T03:00:00Z', uploadBytes: '9007199254740993', downloadBytes: '7' },
    { nodeId: 'removed', hour: '2026-11-04T03:00:00Z', uploadBytes: '11', downloadBytes: '13' },
  ],
}
it('combines only recorded directional bytes and converts only graph coordinates', () => {
  const all = paygHistorySeries(history)
  expect(all).toHaveLength(73)
  expect(all.at(-1)?.uploadExact).toBe('9007199254741004')
  expect(all.at(-1)?.downloadExact).toBe('20')
  expect(all.at(-1)?.upload).toBe(Number('9007199254741004'))
  expect(paygHistorySeries(history, 'current').at(-1)?.uploadExact).toBe('9007199254740993')
  expect(paygHistorySeries(history, 'removed').at(-1)?.downloadExact).toBe('13')
})
