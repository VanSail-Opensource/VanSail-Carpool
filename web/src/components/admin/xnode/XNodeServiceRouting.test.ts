import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { NamedService } from '@/api/xnode'
import type { XNodeCustomization } from '@/api/xnode'
import XNodeServiceRouting from './XNodeServiceRouting.vue'

const { read, save } = vi.hoisted(() => ({ read: vi.fn(), save: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: { serviceRouting: read, saveServiceRouting: save } }))
vi.mock('@/i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('@/i18n')>(), useI18n: () => ({ t: (key: string) => key }), localizedError: (_: unknown, key: string) => key }))
const Modal = { name: 'ModalStub', props: { open: Boolean, close: Boolean, dismissible: Boolean }, emits: ['update:open'], template: '<section v-if="open"><slot name="body" /></section>' }
const Select = { name: 'SelectStub', props: { modelValue: String, disabled: Boolean }, emits: ['update:modelValue'], template: '<div />' }
const Button = { name: 'ButtonStub', props: { label: String, disabled: Boolean, loading: Boolean }, emits: ['click'], template: '<div @click="!disabled && $emit(\'click\')">{{ label }}</div>' }
const Field = { template: '<div><slot /></div>' }
const Json = { name: 'JsonStub', props: { modelValue: Array, label: String, disabled: Boolean }, emits: ['update:modelValue', 'valid'], template: '<div />' }
const base = { Modal, Select, Button, FormField: Field, Alert: true, Progress: true }
const stubs = { ...base, ...Object.fromEntries(Object.entries(base).map(([key, value]) => ['U' + key, value])), JsonDocumentField: Json }
function owner(id: string): NamedService {
  return { identity: { proxyUserId: id, userId: '', username: id, serviceName: id, currentPeriodId: 'period', accessDisabled: false, createdAt: '2026-01-01T00:00:00Z' }, access: { limitBytes: '0', strategy: 'NO_RESET', startedAt: '2026-01-01T00:00:00Z', expiresAt: '2099-01-01T00:00:00Z', groupIds: ['group'], disabled: false } }
}
function document(tag: string): XNodeCustomization {
  return { nodeId: 'node', outbounds: [{ tag }], routing: [{ outboundTag: tag }], revision: 1 }
}
function routing() {
  return mount(XNodeServiceRouting, { props: {
    item: owner('one'), nodes: [{ id: 'node', name: 'Node', enabled: true, multiplier: 1, configId: 'config' }],
    groups: [{ id: 'group', name: 'Group', enabled: true, bindings: [{ nodeId: 'node', inboundTag: 'in' }] }],
  }, global: { stubs } })
}
beforeEach(() => { read.mockReset(); save.mockReset() })

describe('standalone service routing ownership', () => {
  it('reloads for a new owner on the same node and ignores the older owner response', async () => {
    let finish!: (value: XNodeCustomization) => void
    read.mockImplementationOnce(() => new Promise<XNodeCustomization>(resolve => { finish = resolve }))
      .mockResolvedValueOnce(document('two'))
    const wrapper = routing()
    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'node')
    await nextTick()
    expect(read).toHaveBeenCalledWith('one', 'node')

    await wrapper.setProps({ item: owner('two') })
    await flushPromises()
    finish(document('one'))
    await flushPromises()

    expect(read).toHaveBeenLastCalledWith('two', 'node')
    expect(wrapper.findAllComponents(Json)[0]!.props('modelValue')).toEqual([{ tag: 'two' }])
    wrapper.unmount()
  })

  it('clones reactive submissions and preserves the other unsaved section', async () => {
    read.mockResolvedValue(reactive(document('saved')))
    let finish!: (value: XNodeCustomization) => void
    save.mockImplementation(() => new Promise<XNodeCustomization>(resolve => { finish = resolve }))
    const wrapper = routing()
    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'node')
    await flushPromises()
    const fields = wrapper.findAllComponents(Json)
    const outbounds = reactive([{ tag: 'new-outbound' }])
    fields[0]!.vm.$emit('update:modelValue', outbounds)
    fields[1]!.vm.$emit('update:modelValue', [{ outboundTag: 'unsaved-rule' }])
    await nextTick()
    await wrapper.findAllComponents(Button).find(item => item.props('label') === 'memberNodes.saveOutbounds')!.trigger('click')

    expect(save).toHaveBeenCalledWith('one', 'node', 'outbounds', expect.objectContaining({ outbounds: [{ tag: 'new-outbound' }] }))
    outbounds[0]!.tag = 'changed-after-submit'
    expect(save.mock.calls[0]![3].outbounds).toEqual([{ tag: 'new-outbound' }])
    expect(wrapper.findComponent(Modal).props('dismissible')).toBe(false)
    wrapper.findComponent(Modal).vm.$emit('update:open', false)
    expect(wrapper.emitted('close')).toBeUndefined()

    finish({ ...document('saved'), outbounds: [{ tag: 'new-outbound' }], revision: 2 })
    await flushPromises()
    expect(wrapper.findAllComponents(Json)[1]!.props('modelValue')).toEqual([{ outboundTag: 'unsaved-rule' }])
    wrapper.unmount()
  })

  it('does not replace a new owner editor with a late save result', async () => {
    read.mockResolvedValueOnce(document('one')).mockResolvedValueOnce(document('two'))
    let finish!: (value: XNodeCustomization) => void
    save.mockImplementation(() => new Promise<XNodeCustomization>(resolve => { finish = resolve }))
    const wrapper = routing()
    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'node')
    await flushPromises()
    await wrapper.findAllComponents(Button)[0]!.trigger('click')

    await wrapper.setProps({ item: owner('two') })
    await flushPromises()
    finish(document('obsolete-save'))
    await flushPromises()

    expect(wrapper.findAllComponents(Json)[0]!.props('modelValue')).toEqual([{ tag: 'two' }])
    wrapper.unmount()
  })
})
