/* eslint-disable vue/one-component-per-file */
import { defineComponent, h, type PropType } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { XNode, XNodeConfiguration, XNodeEndpoint, XNodeEndpointPreview } from '@/api/xnode'
const api = vi.hoisted(() => ({ previewEndpoint: vi.fn(), saveEndpoint: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }), t: (key: string) => key, localizedError: (_error: unknown, key: string) => key }))
import XNodeEndpointForm from './XNodeEndpointForm.vue'
const nodes: XNode[] = ['one', 'two'].map(id => ({ id, name: id, configId: id, enabled: true, multiplier: 1 }))
const configurations: XNodeConfiguration[] = nodes.map(node => ({ id: node.id, name: node.id, revision: 1, snippetIds: [], document: { inbounds: [{ tag: 'raw-' + node.id, protocol: 'vless' }] } }))
const endpoint: XNodeEndpoint = { id: 'endpoint', nodeId: 'one', inboundTag: 'compiled-tag', name: 'Saved entry', address: 'proxy.example', port: 443, enabled: true, hidden: false, overrides: { legacyHost: { retained: true } } }
const response = (valid = true): XNodeEndpointPreview => ({ valid, metadata: { protocol: 'vless', transport: 'xhttp', security: 'tls' }, inbounds: [{ tag: 'compiled-tag', protocol: 'vless', transport: 'xhttp', security: 'tls' }], fields: [], clients: [], fieldErrors: valid ? [] : [{ field: 'name', code: 'required' }] })
const stubs = {
  UFormField: defineComponent({ props: { label: { type: String, default: '' }, error: { type: String, default: '' } }, setup: (props, { slots }) => () => h('div', { 'data-label': props.label }, [slots.default?.(), props.error]) }),
  UInput: defineComponent({ props: { modelValue: { type: [String, Number], default: '' } }, emits: ['update:modelValue'], setup: (props, { emit }) => () => h('input', { value: props.modelValue, onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value) }) }),
  USelect: defineComponent({ props: { modelValue: { type: String, default: '' }, items: { type: Array as PropType<{ label: string; value: string }[]>, default: () => [] } }, emits: ['update:modelValue'], setup: (props, { emit }) => () => h('select', { value: props.modelValue, onChange: (event: Event) => emit('update:modelValue', (event.target as HTMLSelectElement).value) }, props.items.map(item => h('option', { value: item.value }, item.label))) }),
  UButton: defineComponent({ props: { label: { type: String, default: '' }, disabled: Boolean, type: { type: String, default: 'button' } }, setup: props => () => h('button', { disabled: props.disabled, type: props.type }, props.label) }),
  UAlert: defineComponent({ props: { title: { type: String, default: '' } }, setup: (props, { slots }) => () => h('div', [props.title, slots.actions?.()]) }),
  USwitch: { template: '<span />' }, XNodeEndpointOptions: { template: '<span />' }, XNodeEndpointCompatibility: { template: '<span />' },
}
const editor = (perform: (action: () => Promise<unknown>) => Promise<boolean> = async action => { await action(); return true }) => mount(XNodeEndpointForm, { props: { item: endpoint, nodes, configurations, busy: false, perform }, global: { stubs } })
beforeEach(() => { vi.resetAllMocks(); vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }); api.previewEndpoint.mockResolvedValue(response()); api.saveEndpoint.mockResolvedValue(endpoint) })
afterEach(() => vi.useRealTimers())
const settle = async () => { await vi.advanceTimersByTimeAsync(300); await flushPromises() }

it('retains the selected compiled inbound while loading or failing and uses the authoritative list', async () => {
  api.previewEndpoint.mockRejectedValueOnce(new Error('offline'))
  const wrapper = editor()
  expect(wrapper.get('[data-label="xnode.inboundTag"] select').element).toHaveProperty('value', 'compiled-tag')
  await settle()
  expect(wrapper.text()).toContain('endpointOptions.previewFailed')
  expect(wrapper.get('[data-label="xnode.inboundTag"] select').element).toHaveProperty('value', 'compiled-tag')
  await wrapper.findAll('button').find(button => button.text() === 'common.tryAgain')!.trigger('click'); await flushPromises()
  expect(wrapper.findAll('[data-label="xnode.inboundTag"] option').map(option => option.attributes('value'))).toEqual(['compiled-tag'])
  await wrapper.get('[data-label="xnode.node"] select').setValue('two')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveEndpoint).not.toHaveBeenCalled()
  wrapper.unmount()
})
it('gates invalid previews and retains the entire draft after save refusal', async () => {
  api.previewEndpoint.mockResolvedValueOnce(response(false))
  const wrapper = editor(async action => { await action(); return false })
  await settle()
  await wrapper.get('form').trigger('submit')
  expect(api.saveEndpoint).not.toHaveBeenCalled()
  expect(wrapper.text()).toContain('endpointOptions.errors.required')
  await wrapper.get('[data-label="common.name"] input').setValue('Edited entry')
  await settle()
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveEndpoint.mock.calls[0]?.[0]).toMatchObject({ name: 'Edited entry', inboundTag: 'compiled-tag', overrides: { legacyHost: { retained: true } } })
  expect(api.saveEndpoint.mock.calls[0]?.[0]).not.toHaveProperty('metadata')
  expect(api.previewEndpoint).toHaveBeenCalledTimes(3)
  expect(wrapper.emitted('saved')).toBeUndefined()
  expect(wrapper.get('[data-label="common.name"] input').element).toHaveProperty('value', 'Edited entry')
  expect(endpoint.name).toBe('Saved entry')
  await wrapper.setProps({ saveError: 'xnode.saveFailed' })
  expect(wrapper.text()).toContain('endpointOptions.saveFailed')
  expect(wrapper.text()).not.toContain('xnode.saveFailed')
  wrapper.unmount()
})
