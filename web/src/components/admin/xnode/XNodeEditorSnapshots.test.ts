import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { XNodeConfiguration, XNodeGroup, XNodeSnippet } from '@/api/xnode'
import XNodeConfigurations from './XNodeConfigurations.vue'
import XNodeAccessGroups from './XNodeAccessGroups.vue'

const api = vi.hoisted(() => ({ saveConfiguration: vi.fn(), saveSnippet: vi.fn(), saveGroup: vi.fn(), validate: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
vi.mock('@/i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('@/i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
const Modal = { name: 'ModalStub', props: { open: Boolean, close: Boolean, dismissible: Boolean }, emits: ['update:open'], template: '<section v-if="open"><slot name="body" /></section>' }
const Button = { name: 'ButtonStub', props: { label: String, icon: String, disabled: Boolean, loading: Boolean, type: String }, emits: ['click'], template: '<div @click="!disabled && $emit(\'click\')">{{ label }}</div>' }
const Input = { name: 'InputStub', props: { modelValue: String, disabled: Boolean }, emits: ['update:modelValue'], template: '<div />' }
const Field = { template: '<div><slot /></div>' }
const Json = { name: 'JsonStub', props: { modelValue: [Object, Array], disabled: Boolean }, emits: ['update:modelValue', 'valid'], template: '<div />' }
const Bindings = { name: 'BindingsStub', props: { modelValue: Array, disabled: Boolean }, emits: ['update:modelValue', 'valid'], template: '<div />' }
const base = { Modal, Button, Input, FormField: Field, Select: true, SelectMenu: true, Switch: true, Alert: true, Progress: true }
const stubs = { ...base, ...Object.fromEntries(Object.entries(base).map(([key, value]) => ['U' + key, value])), JsonDocumentField: Json, XNodeJsonEditor: Json, XNodeAccessBindings: Bindings, XNodeEndpointForm: true }
beforeEach(() => { for (const mock of Object.values(api)) mock.mockReset() })

describe('native editor submit snapshots', () => {
  it('opens a reactive snippet and captures its payload before a deferred mutation runner', async () => {
    const snippet = reactive<XNodeSnippet>({ id: 'snippet', name: 'Original', kind: 'outbound', revision: 4, document: [{ tag: 'original' }] })
    let run!: () => Promise<unknown>
    let finish!: (value: boolean) => void
    const perform = vi.fn((action: () => Promise<unknown>) => { run = action; return new Promise<boolean>(resolve => { finish = resolve }) })
    const wrapper = mount(XNodeConfigurations, { props: { snippets: [snippet], configurations: [] as XNodeConfiguration[], busy: false, perform }, global: { stubs } })
    await wrapper.findAllComponents(Button).find(item => item.props('icon') === 'i-ph-pencil-simple-bold')!.trigger('click')
    expect(wrapper.findComponent(Json).props('modelValue')).toEqual([{ tag: 'original' }])
    expect(wrapper.findComponent(Json).props('modelValue')).not.toBe(snippet.document)
    await wrapper.get('form').trigger('submit')
    expect(wrapper.findComponent(Modal).props('dismissible')).toBe(false)
    expect(wrapper.findComponent(Input).props('disabled')).toBe(true)
    wrapper.findComponent(Modal).vm.$emit('update:open', false)
    wrapper.findComponent(Input).vm.$emit('update:modelValue', 'Late change')
    wrapper.findComponent(Json).vm.$emit('update:modelValue', [{ tag: 'late-change' }])
    await nextTick()
    await run()

    expect(api.saveSnippet).toHaveBeenCalledWith({ id: 'snippet', name: 'Original', kind: 'outbound', revision: 4, document: [{ tag: 'original' }] })
    expect(wrapper.get('form').exists()).toBe(true)
    finish(true)
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(false)
    wrapper.unmount()
  })

  it('clones reactive group bindings and submits only editable group fields', async () => {
    const group = reactive<XNodeGroup>({ id: 'group', name: 'Original', enabled: true, bindings: [{ nodeId: 'node', inboundTag: 'in' }], memberCount: 3, canDelete: false })
    let run!: () => Promise<unknown>
    let finish!: (value: boolean) => void
    const perform = vi.fn((action: () => Promise<unknown>) => { run = action; return new Promise<boolean>(resolve => { finish = resolve }) })
    const wrapper = mount(XNodeAccessGroups, { props: { groups: [group], endpoints: [], nodes: [], configurations: [], busy: false, perform }, global: { stubs } })
    await wrapper.findAllComponents(Button).find(item => item.props('icon') === 'i-ph-pencil-simple-bold')!.trigger('click')
    expect(wrapper.findComponent(Bindings).props('modelValue')).toEqual(group.bindings)
    expect(wrapper.findComponent(Bindings).props('modelValue')).not.toBe(group.bindings)
    await wrapper.get('form').trigger('submit')
    expect(wrapper.findComponent(Bindings).props('disabled')).toBe(true)
    wrapper.findComponent(Input).vm.$emit('update:modelValue', 'Late change')
    wrapper.findComponent(Bindings).vm.$emit('update:modelValue', [])
    await run()

    expect(api.saveGroup).toHaveBeenCalledWith({ id: 'group', name: 'Original', enabled: true, bindings: [{ nodeId: 'node', inboundTag: 'in' }] })
    finish(false)
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(true)
    wrapper.unmount()
  })
})
