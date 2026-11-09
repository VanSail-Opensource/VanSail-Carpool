/* eslint-disable vue/one-component-per-file -- Colocated dependency stubs exercise one real chart component. */
import { mount } from '@vue/test-utils'
import { defineComponent, h, type PropType } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { VisDonut, VisSingleContainer } from '@unovis/vue'
import { usageBreakdownFixture } from '../../../fixtures/usage-breakdown-data'
import { setLocale, t } from '@/i18n'
import { setExperience } from '@/utils/experience'
import { usageSegmentColor, usageSegmentID, usageSegmentValue } from './usageBreakdownSegments'
import UsageBreakdownPie from './UsageBreakdownPie.vue'

const feedback = vi.hoisted(() => vi.fn())
vi.mock('@/utils/telegramHaptics', async importOriginal => ({ ...await importOriginal<typeof import('@/utils/telegramHaptics')>(), selectionHaptic: feedback }))
vi.mock('@unovis/vue', async importOriginal => {
  const actual = await importOriginal<typeof import('@unovis/vue')>()
  const { defineComponent, h } = await import('vue')
  return { ...actual,
    VisSingleContainer: defineComponent({ name: 'VisSingleContainer', props: { data: { type: Array, required: true }, height: { type: Number, required: true }, duration: { type: Number, required: true } }, setup: (_, { slots }) => () => h('svg', { 'data-chart-svg': '' }, slots.default?.()) }),
    VisDonut: defineComponent({ name: 'VisDonut', props: { id: { type: Function, required: true }, value: { type: Function, required: true }, color: { type: Function, required: true }, duration: { type: Number, required: true }, arcWidth: { type: Number, required: true }, events: { type: Object, required: true } }, setup: () => () => h('g') }),
  }
})

const Tabs = defineComponent({
  name: 'UTabs', props: { items: { type: Array as PropType<{ value: string; label: string }[]>, required: true }, modelValue: { type: String, required: true } }, emits: ['update:modelValue'],
  setup: (props, { emit }) => () => h('div', props.items?.map(item => h('button', { 'data-metric': item.value, role: 'tab', onClick: () => emit('update:modelValue', item.value) }, item.label))),
})
const Button = defineComponent({ name: 'UButton', inheritAttrs: false, emits: ['click'], setup: (_, { slots, attrs, emit }) => () => h('button', { ...attrs, onClick: () => emit('click') }, slots.default?.()) })
const cleanups: (() => void)[] = []
function render(mode = '') {
  // Happy DOM computes v-show visibility only for connected elements.
  const target = document.body.appendChild(document.createElement('div'))
  // Nuxt auto-imported SFCs expose their file names to Vue Test Utils.
  const view = mount(UsageBreakdownPie, { attachTo: target, props: { history: usageBreakdownFixture(new URLSearchParams(mode)) }, global: { stubs: { Tabs, UTabs: Tabs, Button, UButton: Button } } })
  cleanups.push(() => { view.unmount(); target.remove() })
  return view
}
afterEach(() => { cleanups.splice(0).forEach(cleanup => cleanup()); vi.clearAllMocks(); setLocale('en'); setExperience('miniapp') })

describe('Usage Breakdown mode stability', () => {
  it('keeps the same SVG through twenty mode changes, including zero traffic and one 100% hourly cost', async () => {
    const view = render('hourly-idle')
    const svg = view.get('[data-chart-svg]').element
    expect(view.get('[data-usage-chart]').isVisible()).toBe(false)
    const donut = view.getComponent(VisDonut)
    expect(donut.props()).toMatchObject({ duration: 0, id: usageSegmentID, value: usageSegmentValue, color: usageSegmentColor })
    expect(view.getComponent(VisSingleContainer).props()).toMatchObject({ height: 220, duration: 0 })
    for (let index = 0; index < 20; index++) {
      await view.get(`[data-metric="${index % 2 === 0 ? 'cost' : 'traffic'}"]`).trigger('click')
      expect(view.get('[data-chart-svg]').element).toBe(svg)
      expect(view.get('[data-usage-chart]').isVisible()).toBe(index % 2 === 0)
    }
    expect(feedback).toHaveBeenCalledTimes(20)
  })
  it('keeps the chart selectable when the original fixed-term duration is unknown', async () => {
    const view = render('unknown-term')
    await view.get('[data-metric="cost"]').trigger('click')
    expect(view.get('[data-usage-chart]').isVisible()).toBe(true)
  })
  it('retains unattributed cost and labels tiny positive proportions without claiming zero', async () => {
    const unattributed = render('unallocated')
    await unattributed.get('[data-metric="cost"]').trigger('click')
    expect(unattributed.get('[data-usage-no-comparison]').text()).toBe(t('usageBreakdown.noAttribution'))
    expect(unattributed.get('[data-usage-chart]').isVisible()).toBe(false)
    const tiny = render('tiny')
    await tiny.get('[data-metric="cost"]').trigger('click')
    expect(tiny.text()).toContain(t('usageBreakdown.lessThanPercent', { amount: '0.1%' }))
  })
  it('shows a single hourly squad as 100% and keeps a small squad named in both legends', async () => {
    const single = render('hourly-one')
    expect(single.get('[data-usage-chart]').isVisible()).toBe(true)
    expect(single.text()).toContain('100%')
    expect(single.text()).toContain(t('usageBreakdown.hourlySquad'))
    await single.get('[data-metric="cost"]').trigger('click')
    expect(single.text()).toContain('100%')
    const small = render('hourly-small')
    expect(small.text()).toContain('Small but named hourly squad')
    await small.get('[data-metric="cost"]').trigger('click')
    expect(small.text()).toContain('Small but named hourly squad')
    expect(small.text()).toContain(t('usageBreakdown.lessThanPercent', { amount: '0.1%' }))
  })
  it('never calls Telegram feedback in a browser experience', async () => {
    setExperience('browser')
    const view = render()
    await view.get('[data-metric="cost"]').trigger('click')
    await view.get('[data-metric="cost"]').trigger('click')
    expect(feedback).not.toHaveBeenCalled()
  })
})
