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
function render(mode = '') {
  return mount(UsageBreakdownPie, { props: { history: usageBreakdownFixture(new URLSearchParams(mode)) }, global: { stubs: { UTabs: Tabs, UButton: Button } } })
}
afterEach(() => { vi.clearAllMocks(); setLocale('en'); setExperience('miniapp') })

describe('Usage Breakdown mode stability', () => {
  it('keeps the same SVG through twenty mode changes, including a hidden one-node measure', async () => {
    const view = render('one-traffic')
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
    view.unmount()
  })
  it('keeps actual cost selectable when the original fixed-term duration is unknown', async () => {
    const view = render('unknown-term')
    await view.get('[data-metric="cost"]').trigger('click')
    expect(view.get('[data-usage-cost-notes]').text()).toContain(t('usageBreakdown.legacyUnknown'))
    expect(view.get('[data-usage-chart]').isVisible()).toBe(true)
    view.unmount()
  })
  it('retains unattributed cost and labels tiny positive proportions without claiming zero', async () => {
    const unattributed = render('unallocated')
    await unattributed.get('[data-metric="cost"]').trigger('click')
    expect(unattributed.get('[data-usage-no-comparison]').text()).toBe(t('usageBreakdown.noNodeAttribution'))
    expect(unattributed.get('[data-usage-chart]').isVisible()).toBe(false)
    unattributed.unmount()
    const tiny = render('tiny')
    await tiny.get('[data-metric="cost"]').trigger('click')
    expect(tiny.text()).toContain(t('usageBreakdown.lessThanPercent', { amount: '0.1%' }))
    tiny.unmount()
  })
  it('never calls Telegram feedback in a browser experience', async () => {
    setExperience('browser')
    const view = render()
    await view.get('[data-metric="cost"]').trigger('click')
    await view.get('[data-metric="cost"]').trigger('click')
    expect(feedback).not.toHaveBeenCalled()
    view.unmount()
  })
})
