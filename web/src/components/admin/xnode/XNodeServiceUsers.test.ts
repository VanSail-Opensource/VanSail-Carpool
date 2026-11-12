import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { NamedService } from '@/api/xnode'
import XNodeServiceUsers from './XNodeServiceUsers.vue'

const api = vi.hoisted(() => ({ namedServices: vi.fn(), saveNamedService: vi.fn(), serviceSubscription: vi.fn(), rotateServiceSubscription: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
vi.mock('@/i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/i18n')>(),
  useI18n: () => ({ t: (key: string, values?: Record<string, string>) => values?.name ? key + ':' + values.name : key }),
  localizedError: (_: unknown, key: string) => key,
}))
const Button = { name: 'ButtonStub', props: { label: String, icon: String, disabled: Boolean, loading: Boolean }, emits: ['click'], template: '<div @click="!disabled && $emit(\'click\')">{{ label }}</div>' }
const Field = { name: 'FieldStub', props: { label: String }, template: '<label>{{ label }}<slot /></label>' }
const Input = { name: 'InputStub', props: { modelValue: String }, template: '<span>{{ modelValue }}</span>' }
const Editor = { name: 'EditorStub', props: { item: Object, busy: Boolean }, emits: ['save', 'close'], template: '<div />' }
const base = { Button, FormField: Field, Input, Alert: true, Progress: true }
const stubs = { ...base, ...Object.fromEntries(Object.entries(base).map(([key, value]) => ['U' + key, value])), XNodeServiceEditor: Editor, XNodeServiceRouting: true }
function owner(id: string): NamedService {
  return { identity: { proxyUserId: id, userId: '', username: id, serviceName: id, currentPeriodId: 'period', accessDisabled: false, createdAt: '2026-01-01T00:00:00Z' }, access: { limitBytes: '0', strategy: 'NO_RESET', startedAt: '2026-01-01T00:00:00Z', expiresAt: '2099-01-01T00:00:00Z', groupIds: ['group'], disabled: false } }
}
function users() {
  return mount(XNodeServiceUsers, { props: { nodes: [], groups: [{ id: 'group', name: 'Group', enabled: true, bindings: [] }] }, global: { stubs } })
}
beforeEach(() => { for (const mock of Object.values(api)) mock.mockReset() })

describe('named service editor lifecycle', () => {
  it('ignores an older list response and labels a returned subscription by owner', async () => {
    let finish!: (value: { items: NamedService[] }) => void
    const initial = owner('initial')
    api.namedServices.mockResolvedValueOnce({ items: [initial] })
      .mockImplementationOnce(() => new Promise<{ items: NamedService[] }>(resolve => { finish = resolve }))
      .mockResolvedValueOnce({ items: [owner('current')] })
    api.saveNamedService.mockResolvedValue(initial)
    api.serviceSubscription.mockResolvedValue({ url: '/subscription/current' })
    const wrapper = users()
    await flushPromises()
    await wrapper.findAllComponents(Button).find(item => item.props('icon') === 'i-ph-arrow-clockwise')!.trigger('click')
    await wrapper.findAllComponents(Button).find(item => item.props('icon') === 'i-ph-pencil-simple-bold')!.trigger('click')
    wrapper.findComponent(Editor).vm.$emit('save', { name: 'initial', access: initial.access })
    await flushPromises()
    finish({ items: [owner('obsolete')] })
    await flushPromises()
    expect(wrapper.text()).toContain('current')
    expect(wrapper.text()).not.toContain('obsolete')

    await wrapper.findAllComponents(Button).find(item => item.props('icon') === 'i-ph-link-bold')!.trigger('click')
    await flushPromises()
    expect(wrapper.findComponent(Field).props('label')).toBe('namedServices.subscriptionFor:current')
    expect(wrapper.findComponent(Input).props('modelValue')).toBe('/subscription/current')
    wrapper.unmount()
  })

  it('keeps a saving editor open and does not reload after its owner view unmounts', async () => {
    const item = owner('one')
    api.namedServices.mockResolvedValue({ items: [item] })
    let finish!: (value: NamedService) => void
    api.saveNamedService.mockImplementation(() => new Promise<NamedService>(resolve => { finish = resolve }))
    const wrapper = users()
    await flushPromises()
    await wrapper.findAllComponents(Button).find(button => button.props('icon') === 'i-ph-pencil-simple-bold')!.trigger('click')
    const editor = wrapper.findComponent(Editor)
    editor.vm.$emit('save', { name: 'one', access: item.access })
    await flushPromises()
    editor.vm.$emit('close')
    await flushPromises()
    expect(wrapper.findComponent(Editor).exists()).toBe(true)
    expect(api.saveNamedService).toHaveBeenCalledWith('one', expect.objectContaining({ name: 'one' }))

    wrapper.unmount()
    finish(item)
    await flushPromises()
    expect(api.namedServices).toHaveBeenCalledTimes(1)
  })
})
