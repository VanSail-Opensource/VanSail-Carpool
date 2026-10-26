import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { StatisticsSnapshot } from '@/api/types'
import StatisticsFreshness from './StatisticsFreshness.vue'

const zeroMoney = { currency: 'TXB', minor: '0', display: '0.00 TXB' } as const
const snapshot: StatisticsSnapshot = {
  generatedAt: '2026-08-18T02:00:00Z',
  remoteGeneratedAt: '2026-08-18T01:30:00Z',
  databaseGeneratedAt: '2026-08-18T02:00:00Z',
  stalePartitions: ['remote'],
  remote: {
    weeklyUserIncrease: 0,
    monthlyAverageUsagePercent: 0,
    trafficDates: [],
    trafficSeries: [],
  },
  database: {
    newUserConversionPercent: 0,
    averageSpend: zeroMoney,
    spendMinimum: zeroMoney,
    spendMaximum: zeroMoney,
    subscriptionStates: [],
    averageCheckInReward: zeroMoney,
    comboShares: [],
    groupMessagesTotal: 0,
    averageOptionalSquads: 0,
    paymentStatuses: [],
    databaseBytes: '0',
    squadByCombo: [],
    comboBySquad: [],
  },
}

describe('StatisticsFreshness', () => {
  it('shows each partition timestamp with its own freshness state', () => {
    const wrapper = mount(StatisticsFreshness, { props: { snapshot } })
    const entries = wrapper.findAll('.statistics-freshness')

    expect(entries).toHaveLength(2)
    expect(entries[0]?.text()).toContain('Last known')
    expect(entries[0]?.find('time').attributes('datetime')).toBe(snapshot.remoteGeneratedAt)
    expect(entries[1]?.text()).toContain('Current')
    expect(entries[1]?.find('time').attributes('datetime')).toBe(snapshot.databaseGeneratedAt)
  })
})
