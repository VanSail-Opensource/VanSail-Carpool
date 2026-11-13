import { defineComponent, h, type FunctionalComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { HourlyPolicySnapshot } from '@/api/hourly'
import HourlyAdminPage from './HourlyAdminPage.vue'
import HourlyPolicyForm from './HourlyPolicyForm.vue'
import AdminSectionState from '../AdminSectionState.vue'

const api = vi.hoisted(() => ({ policy: vi.fn(), offers: vi.fn(), savePolicy: vi.fn(), saveOffer: vi.fn(), getAdminResource: vi.fn() }))
vi.mock('@/api/hourly', () => ({ hourlyApi: api }))
vi.mock('@/api/client', () => ({ api }))
const snapshot: HourlyPolicySnapshot = {
  version: 'read-version', policy: { salesEnabled: false, dailyAllowanceBytes: '1000', reducedRateBytesPerSecond: '1000', extraGBPriceTXBMinor: '100', activationHours: 1, cooldownSeconds: 0 },
}
const button = defineComponent({ props: { label: { type: String, default: '' }, disabled: Boolean }, emits: ['click'], setup: (props, { emit }) => () => h('button', { disabled: props.disabled, onClick: () => emit('click') }, props.label) })
const passthrough: FunctionalComponent = (_, { slots }) => h('div', slots.default?.())
function page() { return mount(HourlyAdminPage, { global: { stubs: { Button: button, UButton: button, SkeletonBlock: true, InlineNotice: passthrough, HourlyPolicyForm: true, HourlyOfferForm: true } } }) }
beforeEach(() => {
  for (const mock of Object.values(api)) mock.mockReset()
  api.policy.mockResolvedValue(snapshot)
  api.offers.mockResolvedValue([])
  api.getAdminResource.mockResolvedValue({ items: [] })
})

describe('hourly administration read/write ownership', () => {
  it('blocks retained-content edits during refresh and excludes reads during a save', async () => {
    const wrapper = page()
    await flushPromises()
    let finishRead!: (value: HourlyPolicySnapshot) => void
    api.policy.mockReturnValueOnce(new Promise(resolve => { finishRead = resolve }))
    await wrapper.get('button').trigger('click')
    const form = wrapper.findComponent(HourlyPolicyForm)
    expect(form.props('disabled')).toBe(true)
    form.vm.$emit('save', snapshot.policy)
    await flushPromises()
    expect(api.savePolicy).not.toHaveBeenCalled()
    finishRead({ ...snapshot, version: 'reviewed-version' })
    await flushPromises()
    expect(form.props('disabled')).toBe(false)
    let finishSave!: (value: HourlyPolicySnapshot) => void
    api.savePolicy.mockReturnValueOnce(new Promise(resolve => { finishSave = resolve }))
    form.vm.$emit('save', snapshot.policy)
    await flushPromises()
    wrapper.findComponent(AdminSectionState).vm.$emit('retry')
    await flushPromises()
    expect(api.policy).toHaveBeenCalledTimes(2)
    expect(api.savePolicy).toHaveBeenCalledWith('reviewed-version', snapshot.policy)
    finishSave({ ...snapshot, version: 'saved-version' })
    await flushPromises()
    expect(form.props('snapshot').version).toBe('saved-version')
    wrapper.unmount()
  })
  it('requires recovery of a failed refresh before overwriting retained configuration', async () => {
    const wrapper = page()
    await flushPromises()
    api.policy.mockRejectedValueOnce(new Error('unavailable'))
    await wrapper.get('button').trigger('click')
    await flushPromises()
    const form = wrapper.findComponent(HourlyPolicyForm)
    expect(form.props('snapshot').version).toBe('read-version')
    expect(form.props('disabled')).toBe(true)
    form.vm.$emit('save', snapshot.policy)
    await flushPromises()
    expect(api.savePolicy).not.toHaveBeenCalled()
    wrapper.findComponent(AdminSectionState).vm.$emit('retry')
    await flushPromises()
    expect(form.props('disabled')).toBe(false)
    wrapper.unmount()
  })
})
