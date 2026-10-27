import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ChartTooltipContent from './ChartTooltipContent.vue'
import { bridgeSeries } from './bridge'

describe('daily chart gaps', () => {
  it('connects the last available actual to the next forecast without filling missing days', () => {
    const rows = [
      { x: 1, label: 'day 1', actual: 4 },
      { x: 2, label: 'day 2', actual: undefined },
      { x: 4, label: 'day 4', predicted: 9 },
    ]
    const bridge = bridgeSeries(rows, 'actual', 'predicted')
    expect(bridge.map(point => [point.x, point.bridgeValue])).toEqual([[1, 4], [4, 9]])
    expect(rows[1]?.actual).toBeUndefined()
    expect(bridgeSeries(rows.slice(0, 2), 'actual', 'predicted')).toEqual([])
  })

  it('distinguishes unavailable values from zero in inspection tooltips', () => {
    const wrapper = mount(ChartTooltipContent, { props: {
      config: { actual: { label: 'Actual', color: 'red' }, predicted: { label: 'Forecast', color: 'blue' } },
      payload: { actual: 0 }, missingLabel: 'Unavailable',
    } })
    expect(wrapper.text()).toContain('Actual0')
    expect(wrapper.text()).toContain('ForecastUnavailable')
  })
})
