import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { IPLookupBGPTopology } from '@/api/ipLookup'
import { t } from '@/i18n'
import IPBGPTopology from './IPBGPTopology.vue'

const ButtonStub = defineComponent({
  inheritAttrs: false,
  props: { label: { type: String, default: '' }, color: { type: String, default: '' },
    variant: { type: String, default: '' }, icon: { type: String, default: '' } },
  setup(props, { attrs }) { return () => h('button', { type: 'button', ...attrs }, props.label) },
})
const topology: IPLookupBGPTopology = {
  status: 'success', source: 'fixture', observedAt: '2026-11-09T00:00:00Z', errorCode: '',
  prefix: '192.0.2.0/24', origins: [64500], nodes: [{ asn: 64500, name: 'Origin' }, { asn: 64501, name: 'Transit' }],
  edges: [{ from: 64501, to: 64500, relationship: 'upstream' }], paths: [], truncated: false,
  relationshipMeta: { status: 'success', source: 'fixture', observedAt: '2026-11-09T00:00:00Z', errorCode: '' },
}
const mountGraph = () => mount(IPBGPTopology, {
  props: { topology },
  global: { stubs: { UButton: ButtonStub, LiveSourceNote: true, IPBGPFocus: true } },
})
beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(700)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(420)
})
afterEach(() => vi.restoreAllMocks())

describe('graph interaction controls', () => {
  it('starts in page-scroll mode, exposes its intent, and exits with Escape', async () => {
    const wrapper = mountGraph()
    const viewport = wrapper.get('.graph-viewport')
    const toggle = wrapper.get('button[aria-pressed]')
    expect(viewport.classes()).not.toContain('graph-viewport--interactive')
    expect(toggle.attributes('aria-pressed')).toBe('false')
    expect(toggle.attributes('aria-controls')).toBe(viewport.attributes('id'))
    expect(wrapper.text()).toContain(t('ipLookupLive.graph.scrollHint'))
    await toggle.trigger('click')
    expect(toggle.attributes('aria-pressed')).toBe('true')
    expect(viewport.classes()).toContain('graph-viewport--interactive')
    await viewport.trigger('keydown', { key: 'Escape' })
    expect(toggle.attributes('aria-pressed')).toBe('false')
    await toggle.trigger('click')
    await wrapper.setProps({ topology: { ...topology, prefix: '198.51.100.0/24' } })
    expect(toggle.attributes('aria-pressed')).toBe('false')
    wrapper.unmount()
  })
  it('keeps toolbar zoom, keyboard panning and node selection available without touch capture', async () => {
    const wrapper = mountGraph()
    const scene = wrapper.get('.graph-scene')
    const fitted = scene.attributes('transform')
    await wrapper.get(`button[aria-label="${t('ipLookupLive.graph.zoomIn')}"]`).trigger('click')
    expect(scene.attributes('transform')).not.toBe(fitted)
    await wrapper.get(`button[aria-label="${t('ipLookupLive.graph.reset')}"]`).trigger('click')
    expect(scene.attributes('transform')).toBe(fitted)
    await wrapper.get('.graph-viewport').trigger('keydown', { key: 'ArrowRight' })
    expect(scene.attributes('transform')).not.toBe(fitted)
    await wrapper.get('[data-asn="64500"]').trigger('keydown', { key: 'Enter' })
    expect(wrapper.get('[data-asn="64500"]').classes()).toContain('graph-selected')
    expect(wrapper.get('button[aria-pressed]').attributes('aria-pressed')).toBe('false')
    wrapper.unmount()
  })
})
