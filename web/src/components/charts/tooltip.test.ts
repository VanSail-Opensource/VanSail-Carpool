import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import ChartTooltipContent from './ChartTooltipContent.vue'
import { componentToString } from './tooltip'

it('preserves exact decimal evidence while numeric plot coordinates round', () => {
  const wrapper = mount(ChartTooltipContent, { props: {
    config: { upload: { label: 'Upload', color: 'blue' } },
    payload: { x: 0, label: 'Hour', upload: Number('9007199254740993'), uploadExact: '9007199254740993' },
    datumValueFormatter: (row, key) => String(row[`${key}Exact`]),
  } })
  expect(wrapper.text()).toContain('9007199254740993')
  expect(wrapper.text()).not.toContain('9007199254740992')
  wrapper.unmount()
})

it('keeps labels and exact payloads escaped in detached tooltip output', () => {
  const render = componentToString({ upload: { label: '<img src=x>', color: 'blue' } }, ChartTooltipContent, { datumValueFormatter: () => '<script>unsafe</script>' })
  const html = render({ x: 0, label: '<svg onload=x>', upload: 1 }, 0)
  expect(html).toContain('&lt;img')
  expect(html).toContain('&lt;script&gt;')
  expect(html).not.toContain('<script>')
})
