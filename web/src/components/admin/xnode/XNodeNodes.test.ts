/* eslint-disable vue/one-component-per-file */

import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h, onMounted } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { XNode, XNodeConfiguration } from '@/api/xnode'

const api = vi.hoisted(() => ({ node: vi.fn(), saveNode: vi.fn(), enroll: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }), localizedError: (_caught: unknown, key: string) => key }))
import XNodeNodes from './XNodeNodes.vue'

const stubs = {
  Modal: defineComponent({ props: { open: Boolean, close: Boolean, dismissible: Boolean }, setup: (props, { slots }) => () => props.open ? h('div', slots.body?.()) : null }),
  FormField: { props: { label: { type: String, default: '' } }, template: '<label :data-label="label"><slot /></label>' },
  Input: defineComponent({ props: { modelValue: { type: String, default: '' } }, emits: ['update:modelValue'], setup: (props, { emit }) => () => h('input', { value: props.modelValue, onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value) }) }),
  Button: defineComponent({ props: { label: { type: String, default: '' }, type: { type: String, default: 'button' }, disabled: Boolean }, setup: props => () => h('button', { type: props.type, disabled: props.disabled }, props.label) }),
  XNodeOperationalSettings: defineComponent({ emits: ['valid'], setup: (_props, { emit }) => { onMounted(() => emit('valid', true)); return () => h('div') } }),
  XNodePricingCostSettings: { template: '<span />' }, XNodeRemovalDialog: { template: '<span />' }, XNodeCommands: { template: '<span />' },
  Select: { template: '<span />' }, Switch: { template: '<span />' }, Badge: { template: '<span><slot /></span>' }, Alert: { template: '<span />' }, Progress: { template: '<span />' },
}
const node: XNode = { id: 'node', name: 'Amsterdam', enabled: true, configId: 'config', countryCode: 'NL', providerName: '', multiplier: 1 }
const configuration: XNodeConfiguration = { id: 'config', name: 'Xray', revision: 1, snippetIds: [], document: { inbounds: [] } }
function editor(nodes: XNode[], perform: (action: () => Promise<unknown>) => Promise<boolean> = async action => { await action(); return true }) {
  return mount(XNodeNodes, { props: { nodes, configurations: [configuration], endpoints: [], metrics: null, busy: false, perform }, global: { stubs } })
}
beforeEach(() => {
  vi.clearAllMocks()
  api.node.mockResolvedValue({ node, settings: { certificates: {}, inboundCertificates: {}, dns: {}, firewall: { enabled: false, protect: false } } })
  api.saveNode.mockResolvedValue(node)
})

describe('retired node traffic multiplier controls', () => {
  it('locks the node editor and captures its submitted name before a deferred mutation runner', async () => {
    let run!: () => Promise<unknown>
    let finish!: (value: boolean) => void
    const perform = vi.fn((action: () => Promise<unknown>) => { run = action; return new Promise<boolean>(resolve => { finish = resolve }) })
    const wrapper = editor([node], perform)
    await wrapper.get('[aria-label="common.edit"]').trigger('click')
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    expect(wrapper.findAllComponents(stubs.Modal)[0]!.props('close')).toBe(false)
    await wrapper.get('[data-label="common.name"] input').setValue('Late change')
    await wrapper.get('form').trigger('submit')
    expect(perform).toHaveBeenCalledOnce()
    await run()
    expect(api.saveNode.mock.calls[0]![0]).toMatchObject({ id: 'node', name: 'Amsterdam' })
    finish(false)
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(true)
    wrapper.unmount()
  })

  it.each([undefined, null, 0, 2.5, 'obsolete', {}, [2]])('omits obsolete multiplier %j from display and node writes', async multiplier => {
    const legacy = { ...node, multiplier } as unknown as XNode
    const wrapper = editor([legacy])
    expect(wrapper.text()).not.toContain('×')
    await wrapper.get('[aria-label="common.edit"]').trigger('click'); await flushPromises()
    expect(wrapper.find('[data-label="xnode.multiplier"]').exists()).toBe(false)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.saveNode).toHaveBeenCalledTimes(1)
    expect(api.saveNode.mock.calls[0]?.[0]).not.toHaveProperty('multiplier')
    expect(legacy.multiplier).toBe(multiplier)
    wrapper.unmount()
  })

  it('creates nodes without introducing a replacement multiplier input', async () => {
    const wrapper = editor([])
    await wrapper.findAll('button').find(button => button.text() === 'xnode.addNode')!.trigger('click')
    await wrapper.get('[data-label="common.name"] input').setValue('Singapore')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.saveNode.mock.calls[0]?.[0]).toMatchObject({ name: 'Singapore', configId: 'config' })
    expect(api.saveNode.mock.calls[0]?.[0]).not.toHaveProperty('multiplier')
    wrapper.unmount()
  })

  it('serializes enrollment creation and ignores the token after another editor opens', async () => {
    let finish!: (value: unknown) => void
    api.enroll.mockReturnValue(new Promise(resolve => { finish = resolve }))
    const wrapper = editor([node, { ...node, id: 'second', name: 'Second' }])
    const actions = wrapper.findAll('[aria-label="xnode.enroll"]')
    await actions[0]!.trigger('click')
    await actions[1]!.trigger('click')
    expect(api.enroll).toHaveBeenCalledExactlyOnceWith('node')
    await wrapper.findAll('[aria-label="common.edit"]')[1]!.trigger('click')
    await flushPromises()
    finish({ token: 'late-first-node-secret', masterUrl: 'https://panel.example', expiresAt: '2026-12-01T00:00:00Z' })
    await flushPromises()
    expect(wrapper.find('[data-label="xnode.enrollmentToken"]').exists()).toBe(false)
    expect(wrapper.html()).not.toContain('late-first-node-secret')
    wrapper.unmount()
  })
})
