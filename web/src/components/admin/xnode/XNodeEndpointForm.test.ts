/* eslint-disable vue/one-component-per-file */
import { defineComponent, h, type PropType } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { XNode, XNodeConfiguration, XNodeEndpoint, XNodeEndpointPreview } from '@/api/xnode'
const api = vi.hoisted(() => ({ previewEndpoint: vi.fn(), saveEndpoint: vi.fn() }))
const messages = vi.hoisted(() => ({ 'endpointOptions.errors.required': 'endpoint-required-error', 'endpointOptions.errors.invalid': 'endpoint-invalid-error' }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
vi.mock('@/i18n', () => {
  const t = (key: string) => messages[key as keyof typeof messages] ?? key
  return { useI18n: () => ({ t }), t, localizedError: (_error: unknown, key: string) => key }
})
import XNodeEndpointForm from './XNodeEndpointForm.vue'
const nodes: XNode[] = ['one', 'two'].map(id => ({ id, name: id, configId: id, enabled: true }))
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
// Nuxt's Vite imports retain bare SFC names; registered aliases may use U names.
const namedStubs = { ...stubs, FormField: stubs.UFormField, Input: stubs.UInput, Select: stubs.USelect, Button: stubs.UButton, Alert: stubs.UAlert, Switch: stubs.USwitch }
const editor = (perform: (action: () => Promise<unknown>) => Promise<boolean> = async action => { await action(); return true }) => mount(XNodeEndpointForm, { props: { item: endpoint, nodes, configurations, busy: false, perform }, global: { stubs: namedStubs } })
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
  expect(wrapper.text()).toContain(messages['endpointOptions.errors.required'])
  expect(wrapper.text()).not.toContain(messages['endpointOptions.errors.invalid'])
  await wrapper.get('[data-label="common.name"] input').setValue('Edited entry')
  await settle()
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveEndpoint.mock.calls[0]?.[0]).toMatchObject({ id: endpoint.id, name: 'Edited entry', inboundTag: 'compiled-tag', overrides: { legacyHost: { retained: true } } })
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

it('retries an uncertain endpoint creation with one private identity and keeps preview drafts new', async () => {
  const rows = new Map<string, Partial<XNodeEndpoint>>()
  let responseLost = true
  api.saveEndpoint.mockImplementation(async (payload: Partial<XNodeEndpoint>) => {
    const id = payload.id || `server-${rows.size + 1}`
    rows.set(id, { ...payload, id })
    if (responseLost) { responseLost = false; throw new Error('response lost after commit') }
    return rows.get(id)
  })
  const perform = async (action: () => Promise<unknown>): Promise<boolean> => {
    try { await action(); return true }
    catch { return false }
  }
  const create = () => mount(XNodeEndpointForm, { props: { nodes, configurations, busy: false, perform }, global: { stubs: namedStubs } })
  const wrapper = create()
  await wrapper.get('[data-label="common.name"] input').setValue('New endpoint')
  await wrapper.get('[data-label="xnode.address"] input').setValue('new.example.com')
  await settle()
  await wrapper.get('[data-label="xnode.inboundTag"] select').setValue('compiled-tag')
  await settle()
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(rows.size).toBe(1)
  expect(wrapper.emitted('saved')).toBeUndefined()
  const firstId = api.saveEndpoint.mock.calls[0]![0].id
  expect(firstId).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i)

  await wrapper.get('[data-label="common.name"] input').setValue('Corrected endpoint')
  await settle()
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveEndpoint.mock.calls[1]![0]).toMatchObject({ id: firstId, name: 'Corrected endpoint', inboundTag: 'compiled-tag' })
  expect(rows.size).toBe(1)
  expect(wrapper.emitted('saved')).toHaveLength(1)
  expect(api.previewEndpoint.mock.calls.every(([body]) => body.id === '')).toBe(true)
  wrapper.unmount()

  const next = create()
  await next.get('[data-label="common.name"] input').setValue('Another endpoint')
  await next.get('[data-label="xnode.address"] input').setValue('another.example.com')
  await settle()
  await next.get('[data-label="xnode.inboundTag"] select').setValue('compiled-tag')
  await settle()
  await next.get('form').trigger('submit'); await flushPromises()
  expect(api.saveEndpoint.mock.calls[2]![0].id).not.toBe(firstId)
  expect(rows.size).toBe(2)
  next.unmount()
})
