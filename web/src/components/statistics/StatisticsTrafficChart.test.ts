import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { StatisticsSnapshot } from '@/api/types'
import LineChart from '@/components/charts/LineChart.vue'
import { t } from '@/i18n'
import StatisticsTrafficChart from './StatisticsTrafficChart.vue'

function remote(values: string[]): StatisticsSnapshot['remote'] {
  return { weeklyUserIncrease: 0, monthlyAverageUsagePercent: 0,
    trafficDates: ['2026-11-11', '2026-11-12', '2026-11-13'],
    trafficSeries: [{ uuid: 'accounted', countryCode: 'US', name: 'Accounted', dailyBytes: values }] }
}
const mountChart = (values: string[]) => mount(StatisticsTrafficChart, { props: { remote: remote(values) }, global: { stubs: { LineChart: true } } })

describe('statistics exact byte readout', () => {
  it('preserves the decimal source beyond safe numeric chart geometry', () => {
    const wrapper = mountChart(['9007199254740993', '10000', '0'])
    const chart = wrapper.findComponent(LineChart)
    const point = chart.props('data')[0]!
    const text = chart.props('formatDatumValue')!(point, 's0')
    expect(text).toContain(t('adminNative.exactBytes', { bytes: '9007199254740993' }))
    expect(text).not.toContain(t('adminNative.exactBytes', { bytes: '9007199254740992' }))
    wrapper.unmount()
  })

  it('keeps malformed and missing buckets unavailable without inventing zero usage', () => {
    const wrapper = mountChart(['invalid', '1'])
    const chart = wrapper.findComponent(LineChart)
    const rows = chart.props('data')
    expect(rows[0]?.s0).toBeUndefined()
    expect(rows[2]?.s0).toBeUndefined()
    expect(chart.props('formatDatumValue')!(rows[0]!, 's0')).toBe(t('common.notAvailable'))
    expect(chart.props('formatDatumValue')!(rows[1]!, 's0')).toContain(t('adminNative.exactBytes', { bytes: '1' }))
    wrapper.unmount()
  })

  it.each([['invalid', 'invalid', 'invalid'], ['-1', '', '1.5']])('does not report unknown observations as an empty traffic history', (...values) => {
    const wrapper = mountChart(values)
    expect(wrapper.findComponent(LineChart).exists()).toBe(false)
    expect(wrapper.text()).toContain(t('common.notAvailable'))
    expect(wrapper.text()).not.toContain(t('statistics.noTraffic'))
    wrapper.unmount()
  })

  it('retains the empty state for observed zero traffic', () => {
    const wrapper = mountChart(['0', '0', '0'])
    expect(wrapper.findComponent(LineChart).exists()).toBe(false)
    expect(wrapper.text()).toContain(t('statistics.noTraffic'))
    wrapper.unmount()
  })
})
