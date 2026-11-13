/* eslint-disable vue/one-component-per-file */
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { beforeEach, expect, it, vi } from 'vitest'
import type { XNodeGroup } from '@/api/xnode'
import XNodeAccessGroups from './XNodeAccessGroups.vue'

const api = vi.hoisted(() => ({ saveGroup: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
vi.mock('@/i18n', async importOriginal => ({ ...await importOriginal<typeof import('@/i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const Modal = { name: 'ModalStub', props: { open: Boolean }, template: '<section v-if="open"><slot name="body" /></section>' }
const Button = defineComponent({ name: 'ButtonStub', props: { label: { type: String, default: '' }, icon: { type: String, default: '' }, disabled: Boolean, type: { type: String, default: 'button' } }, emits: ['click'], setup: (props, { emit }) => () => h('button', { type: props.type, disabled: props.disabled, onClick: () => emit('click') }, props.label) })
const Input = defineComponent({ name: 'InputStub', props: { modelValue: { type: String, default: '' } }, emits: ['update:modelValue'], setup: (props, { emit }) => () => h('input', { value: props.modelValue, onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value) }) })
const Field = { template: '<div><slot /></div>' }
const base = { Modal, Button, Input, FormField: Field, Switch: true, Alert: true }
const stubs = { ...base, ...Object.fromEntries(Object.entries(base).map(([key, value]) => ['U' + key, value])), XNodeAccessBindings: true, XNodeEndpointForm: true }
const perform = async (action: () => Promise<unknown>): Promise<boolean> => {
  try { await action(); return true }
  catch { return false }
}
beforeEach(() => { api.saveGroup.mockReset() })

it('reuses one group identity after a committed response is lost and allocates another for a new editor', async () => {
  const rows = new Map<string, Partial<XNodeGroup>>()
  let responseLost = true
  api.saveGroup.mockImplementation(async (payload: Partial<XNodeGroup>) => {
    const id = payload.id || `server-${rows.size + 1}`
    rows.set(id, { ...payload, id })
    if (responseLost) { responseLost = false; throw new Error('response lost after commit') }
    return rows.get(id)
  })
  const wrapper = mount(XNodeAccessGroups, { props: { groups: [], endpoints: [], nodes: [], configurations: [], busy: false, perform }, global: { stubs } })
  const add = () => wrapper.findAllComponents(Button).find(button => button.props('label') === 'xnode.addGroup')!
  await add().trigger('click')
  await wrapper.get('input').setValue('First group')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(rows.size).toBe(1)
  expect(wrapper.find('form').exists()).toBe(true)
  const firstId = api.saveGroup.mock.calls[0]![0].id
  expect(firstId).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i)

  await wrapper.get('input').setValue('Corrected group')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveGroup.mock.calls[1]![0]).toMatchObject({ id: firstId, name: 'Corrected group' })
  expect(rows.size).toBe(1)
  expect(wrapper.find('form').exists()).toBe(false)

  await add().trigger('click')
  await wrapper.get('input').setValue('Another group')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveGroup.mock.calls[2]![0].id).not.toBe(firstId)
  expect(rows.size).toBe(2)
  wrapper.unmount()
})

it('retains the existing group identity when editing', async () => {
  api.saveGroup.mockResolvedValue({})
  const group: XNodeGroup = { id: 'existing-group', name: 'Existing', enabled: true, bindings: [], memberCount: 0, canDelete: true }
  const wrapper = mount(XNodeAccessGroups, { props: { groups: [group], endpoints: [], nodes: [], configurations: [], busy: false, perform }, global: { stubs } })
  await wrapper.findAllComponents(Button).find(button => button.props('icon') === 'i-ph-pencil-simple-bold')!.trigger('click')
  await wrapper.get('input').setValue('Edited')
  await nextTick()
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveGroup).toHaveBeenCalledWith({ id: 'existing-group', name: 'Edited', enabled: true, bindings: [] })
  expect(group.name).toBe('Existing')
  wrapper.unmount()
})
