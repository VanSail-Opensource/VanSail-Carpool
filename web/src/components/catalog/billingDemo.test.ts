import { describe, expect, it } from 'vitest'
import { hourlyDemoState, trafficDemoState } from './billingDemo'

describe('synthetic billing film boundaries', () => {
  it('keeps wallet debits discrete while traffic counters move', () => {
    expect(trafficDemoState(1, .4).blocks).toBe(0)
    expect(trafficDemoState(1, .7)).toMatchObject({ blocks: 1, usageMiB: 100, creditMiB: 924, balance: '9.00' })
    for (const phase of [0, .2, .5, .8, 1]) expect(trafficDemoState(2, phase).balance).toBe('9.00')
    expect(trafficDemoState(3, .4).blocks).toBe(1)
    expect(trafficDemoState(3, 1)).toMatchObject({ blocks: 2, usageMiB: 1536, creditMiB: 512, balance: '8.00' })
  })

  it('does not invent charges on connection, reconnection or idle expiry', () => {
    expect(trafficDemoState(0, 1).balance).toBe('10.00')
    expect(trafficDemoState(4, 0)).toMatchObject({ connected: false, creditMiB: 512, balance: '8.00' })
    expect(trafficDemoState(4, 1)).toMatchObject({ connected: true, creditMiB: 512, balance: '8.00' })
    expect(trafficDemoState(5, 1)).toMatchObject({ expired: true, creditMiB: 0, balance: '8.00' })
  })

  it('shows consent before an extra purchase and keeps paid time non-refundable', () => {
    expect(hourlyDemoState(0, 0)).toMatchObject({ enabled: false, balance: '10.00' })
    expect(hourlyDemoState(0, 1)).toMatchObject({ enabled: true, balance: '9.50', debit: '0.50' })
    expect(hourlyDemoState(1, 1)).toMatchObject({ balance: '8.50', debit: '1.00' })
    expect(hourlyDemoState(3, 1)).toMatchObject({ reduced: true, autoBuy: false, balance: '8.50' })
    expect(hourlyDemoState(4, .4)).toMatchObject({ autoBuy: true, reduced: true, balance: '8.50' })
    expect(hourlyDemoState(4, 1)).toMatchObject({ autoBuy: true, reduced: false, balance: '8.30', debit: '0.20' })
    expect(hourlyDemoState(5, 1)).toMatchObject({ enabled: false, remaining: 60, balance: '8.30', debit: '0.00' })
  })

  it('expires extras at the UTC reset without re-enabling or refunding the squad', () => {
    expect(hourlyDemoState(6, 0)).toMatchObject({ extra: true, reset: false, remaining: 60, enabled: false })
    expect(hourlyDemoState(6, 1)).toMatchObject({ extra: false, reset: true, remaining: 100, enabled: false, balance: '8.30' })
  })
})
