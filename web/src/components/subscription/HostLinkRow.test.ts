import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SubscriptionHost } from '@/api/subscription'
import HostLinkRow from './HostLinkRow.vue'

const { save } = vi.hoisted(() => ({ save: vi.fn() }))
vi.mock('@/api/subscription', () => ({ subscriptionApi: { saveHostName: save } }))
vi.mock('@/i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('@/i18n')>(), useI18n: () => ({ t: (key: string) => key }), localizedError: (_: unknown, key: string) => key }))
const Modal = { name: 'ModalStub', props: { open: Boolean, close: Boolean, dismissible: Boolean }, emits: ['update:open'], template: '<section v-if="open"><slot name="body" /></section>' }
const Button = { name: 'ButtonStub', props: { label: String, disabled: Boolean, loading: Boolean }, emits: ['click'], template: '<div @click="!disabled && $emit(\'click\')">{{ label }}<slot /></div>' }
const Input = { name: 'InputStub', props: { modelValue: String, disabled: Boolean }, emits: ['update:modelValue'], template: '<div />' }
const base = { Modal, Button, Input, FormField: { template: '<div><slot /></div>' }, Badge: true, Skeleton: true }
const stubs = { ...base, ...Object.fromEntries(Object.entries(base).map(([key, value]) => ['U' + key, value])), CountryFlag: true, InlineNotice: true }
function host(uuid: string): SubscriptionHost {
  return { uuid, name: uuid, publishedName: uuid, nameOverride: '', countryCodes: [], squadUuids: [], link: null, linkErrorCode: '' }
}
beforeEach(() => { save.mockReset() })

describe('host name editor identity', () => {
  it.each(['success', 'failure'])('ignores a previous host save %s after the row is replaced', async (outcome) => {
    let finish!: () => void
    save.mockImplementation(() => new Promise<void>((resolve, reject) => {
      finish = () => outcome === 'success' ? resolve() : reject(new Error('old failure'))
    }))
    const wrapper = mount(HostLinkRow, { props: { host: host('first') }, global: { stubs } })
    await wrapper.get('.host-link__name').trigger('click')
    wrapper.findComponent(Input).vm.$emit('update:modelValue', 'first custom name')
    await wrapper.get('form').trigger('submit')
    expect(save).toHaveBeenCalledWith('first', 'first custom name')
    const modal = wrapper.findAllComponents(Modal)[0]!
    expect(modal.props('dismissible')).toBe(false)
    modal.vm.$emit('update:open', false)
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(true)

    await wrapper.setProps({ host: host('second') })
    await wrapper.get('.host-link__name').trigger('click')
    wrapper.findComponent(Input).vm.$emit('update:modelValue', 'second draft')
    finish()
    await flushPromises()

    expect(wrapper.emitted('saved')).toBeUndefined()
    expect(wrapper.find('form').exists()).toBe(true)
    expect(wrapper.findComponent(Input).props('modelValue')).toBe('second draft')
    expect(wrapper.findComponent(Input).props('disabled')).toBe(false)
    wrapper.unmount()
  })
})
