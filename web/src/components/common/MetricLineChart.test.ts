import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import LineChart from '@/components/charts/LineChart.vue'
import { t } from '@/i18n'
import MetricLineChart from './MetricLineChart.vue'

describe('numeric metric precision disclosure', () => {
  it.each(['bytes', undefined] as const)('does not label unsafe or fractional %s samples as exact bytes', unit => {
    const wrapper = mount(MetricLineChart, { props: { label: 'Constructed metric', unit,
      samples: [{ at: 1, value: 2 ** 53 }, { at: 2, value: 1.5 }, { at: 3, value: 10000 }] }, global: { stubs: { LineChart: true } } })
    const chart = wrapper.findComponent(LineChart)
    const rows = chart.props('data')
    const detail = chart.props('formatDatumValue')!
    expect(detail(rows[0]!, 'actual')).not.toContain(' · ')
    expect(detail(rows[1]!, 'actual')).not.toContain(' · ')
    const raw = t('adminNative.exactBytes', { bytes: '10000' })
    expect(detail(rows[2]!, 'actual')).toContain(unit ? raw : t('statistics.perSecond', { value: raw }))
    wrapper.unmount()
  })
})
