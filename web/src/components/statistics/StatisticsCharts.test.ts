import { VisDonut } from '@unovis/vue'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import UApp from '@nuxt/ui/runtime/components/App.vue'

import type { StatisticsSnapshot } from '@/api/types'
import StatisticsDistribution from './StatisticsDistribution.vue'
import StatisticsDonut from './StatisticsDonut.vue'
import StatisticsPie from './StatisticsPie.vue'

const hapticMocks = vi.hoisted(() => ({ selectionHaptic: vi.fn() }))
vi.mock('@/utils/telegram', () => hapticMocks)

const shares = [
  { id: 'active', label: 'Active', value: 3 },
  { id: 'queued', label: 'Queued', value: 1 },
]

describe('statistical chart interaction contracts', () => {
  afterEach(() => vi.clearAllMocks())

  it('renders an interactive donut with exact-value selection details', async () => {
    const wrapper = mount(StatisticsDonut, {
      props: { items: shares, centerLabel: 'Members', centerValue: '4', chartLabel: 'Member split' },
    })

    expect(wrapper.find('[role="group"]').attributes('aria-label')).toBe('Member split')
    const segments = wrapper.findAll('.statistics-legend li')
    expect(segments).toHaveLength(2)
    expect((wrapper.findComponent(VisDonut).props() as { centralLabel: string }).centralLabel).toBe('4')
    expect(wrapper.find('.statistics-chart-detail').text()).toContain('Active3 / 75%')

    await segments[1]?.trigger('focus')
    expect(wrapper.find('.statistics-chart-detail').text()).toContain('Queued1 / 25%')

    await segments[1]?.trigger('click')
    expect(segments[1]?.attributes('aria-pressed')).toBe('true')
    expect(hapticMocks.selectionHaptic).not.toHaveBeenCalled()
    expect(wrapper.find('[data-haptic]').exists()).toBe(false)
    expect(wrapper.html()).not.toContain('conic-gradient')
  })

  it('keeps the full pie legend accessible', () => {
    const wrapper = mount(StatisticsPie, {
      props: { items: shares, chartLabel: 'Subscription states', sliceLabels: true },
    })

    expect(wrapper.findAll('.statistics-legend li')).toHaveLength(2)
  })

  it('normalizes each DB12 stacked bar and exposes interactive exact values', async () => {
    const zero = { currency: 'TXB', minor: '0', display: '0.00 TXB' } as const
    const database = {
      newUserConversionPercent: 0, averageSpend: zero, spendMinimum: zero, spendMaximum: zero,
      subscriptionStates: [], averageCheckInReward: zero, comboShares: [],
      groupMessagesTotal: 0, averageOptionalSquads: 0, paymentStatuses: [], databaseBytes: '0',
      squadByCombo: [],
      comboBySquad: [{
        id: 'squad-a', label: 'Squad A',
        segments: [{ id: 'combo-a', label: 'Combo A', value: 2 }, { id: 'combo-b', label: 'Combo B', value: 1 }],
      }],
    } satisfies StatisticsSnapshot['database']
    const wrapper = mount(UApp, {
      global: { stubs: { UTabs: true, UIcon: true } },
      slots: { default: () => h(StatisticsDistribution, { database }) },
    })
    const chart = wrapper.findComponent({ name: 'LineChart' })
    const point = chart.props('data')[0]
    expect(point.label).toBe('Squad A')
    expect(point['combo-a']).toBeCloseTo(66.667, 2)
    expect(point['combo-b']).toBeCloseTo(33.333, 2)
    await chart.findAll('button').at(-1)?.trigger('click')
    expect(chart.text()).toContain('Combo B: 33.3%')
  })
})
