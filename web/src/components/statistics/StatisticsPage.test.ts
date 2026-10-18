import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { StatisticsNode, StatisticsNodesSnapshot, StatisticsSnapshot } from '@/api/types'
import StatisticsPage from './StatisticsPage.vue'

const apiMocks = vi.hoisted(() => ({
  getStatistics: vi.fn(),
        getStatisticsNodes: vi.fn(),
}))
const hapticMocks = vi.hoisted(() => ({ selectionHaptic: vi.fn() }))

vi.mock('@/api/client', () => ({ api: apiMocks }))
vi.mock('@/composables/useTelegramBackButton', () => ({ useTelegramBackButton: () => undefined }))
vi.mock('@/utils/telegram', () => hapticMocks)

const node: StatisticsNode = {
  uuid: '373f14bc-089a-4c3a-91c3-3421e7c83367', name: 'Tokyo', countryCode: 'JP', online: true,
  usersOnline: 1, rxBytesPerSec: '2', txBytesPerSec: '3', xrayVersion: '1.0', multiplier: 1,
}
const nodes: StatisticsNodesSnapshot = { generatedAt: '2026-08-19T12:00:00Z', stale: false, nodes: [node] }
const money = { currency: 'TXB', minor: '0', display: '0.00 TXB' } as const
const statistics: StatisticsSnapshot = {
  generatedAt: '2026-08-19T12:00:00Z', remoteGeneratedAt: '2026-08-19T12:00:00Z', databaseGeneratedAt: '2026-08-19T12:00:00Z', stalePartitions: [],
  remote: { weeklyUserIncrease: 0, monthlyAverageUsagePercent: 0, predictedAverageRollover: money, trafficDates: [], trafficSeries: [] },
  database: { newUserConversionPercent: 0, averageSpend: money, spendMinimum: money, spendMaximum: money, subscriptionStates: [], averageCheckInReward: money, comboShares: [], groupMessagesTotal: 0, averageOptionalSquads: 0, paymentStatuses: [], databaseBytes: '0', squadByCombo: [], comboBySquad: [] },
}

const TabsStub = {
  props: ['items', 'modelValue'],
  emits: ['update:modelValue'],
  template: `<div>
    <div v-for="item in items" :key="item.value" role="tab" tabindex="0" :data-testid="'tab-' + item.value" @click="$emit('update:modelValue', item.value)" @keydown.enter="$emit('update:modelValue', item.value)">{{ item.label }}</div>
    <slot :name="modelValue" />
  </div>`,
}

function mountPage() {
  return mount(StatisticsPage, {
    global: {
      stubs: {
        InlineNotice: true, SkeletonBlock: true, StatisticsFreshness: { template: '<div data-testid="freshness" />' },
        StatisticsOverview: { template: '<div data-testid="overview" />' }, StatisticsTrafficChart: { template: '<div data-testid="traffic" />' },
        StatisticsShareCharts: { template: '<div data-testid="distributions" />' }, StatisticsDistribution: { template: '<div data-testid="composition" />' },
        Tabs: TabsStub, Tooltip: true, Button: true,
        StatisticsNodes: { props: ['snapshot'], template: '<div data-testid="nodes" />' },
      },
      mocks: { $t: (key: string) => key },
    },
  })
}

describe('StatisticsPage', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows each statistics section in its own tab', async () => {
    apiMocks.getStatistics.mockResolvedValue(statistics)
    apiMocks.getStatisticsNodes.mockResolvedValue(nodes)
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.find('[data-testid="overview"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="freshness"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="traffic"]').exists()).toBe(false)

    await wrapper.get('[data-testid="tab-traffic"]').trigger('click')

    expect(hapticMocks.selectionHaptic).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="overview"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="freshness"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="traffic"]').exists()).toBe(true)
    wrapper.unmount()
  })

})
