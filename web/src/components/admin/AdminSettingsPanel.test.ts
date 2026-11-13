import { defineComponent, h, shallowRef, type ShallowRef } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminSetting } from '@/api/types'
import { api } from '@/api/client'
import type { ActivitySettings } from '@/api/features'
import AdminActivitySettings from './activity/AdminActivitySettings.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { t } from '@/i18n'

const saves = vi.hoisted(() => ({ emby: vi.fn(), activity: vi.fn(), billing: vi.fn(), profiles: vi.fn() }))
const reads = vi.hoisted(() => ({ getActivity: vi.fn(), saveActivity: vi.fn(), restoreRef: vi.fn() }))
const sectionState = vi.hoisted(() => ({ settings: [] as AdminSetting[], perform: vi.fn() }))
vi.mock('@/composables/useAdminSection', () => ({ useAdminSection: () => ({
  items: shallowRef(sectionState.settings), loading: shallowRef(false), busy: shallowRef(false), error: shallowRef(null),
  load: vi.fn(), perform: sectionState.perform,
}) }))
vi.mock('@/api/features', () => ({ featuresApi: { getAdminActivitySettings: reads.getActivity, saveAdminActivitySettings: reads.saveActivity } }))
vi.mock('@/api/cache/restore', async original => ({ ...await original<typeof import('@/api/cache/restore')>(), restoreRef: reads.restoreRef }))
import AdminSettingsPanel from './AdminSettingsPanel.vue'

const activity: ActivitySettings = { timezone: 'UTC', dailyRewardMinTxb: '0.00', dailyRewardMinTxbMinor: '0',
  dailyRewardMaxTxb: '0.00', dailyRewardMaxTxbMinor: '0', groupMessageThreshold: 0,
  groupMessageRewardTxb: '0.00', groupMessageRewardTxbMinor: '0' }
function section(save: () => Promise<boolean>, method = 'save') {
  return defineComponent({ setup(_, { expose }) { expose({ [method]: save, loading: false }); return () => h('div') } })
}
function render(actualActivity = false) {
  return mount(AdminSettingsPanel, { global: { stubs: {
    AdminSectionState: { template: '<div><slot /></div>' },
    AdminEmbyConnection: section(saves.emby), AdminActivitySettings: actualActivity ? false : section(saves.activity),
    AdminBillingAmountLimits: section(saves.billing), AdminPaymentProfiles: section(saves.profiles, 'saveAll'),
    InlineNotice: { props: ['title'], template: '<div>{{ title }}<slot /></div>' },
  } } })
}
beforeEach(() => {
  sectionState.settings = []
  sectionState.perform.mockReset().mockResolvedValue(true)
  Object.values(saves).forEach(save => save.mockReset().mockResolvedValue(true))
  reads.getActivity.mockReset().mockResolvedValue(activity)
  reads.saveActivity.mockReset().mockResolvedValue(activity)
  reads.restoreRef.mockReset().mockReturnValue(false)
})
afterEach(() => vi.restoreAllMocks())

describe('aggregate settings persistence', () => {
  it('does not dispatch a later enable stage after the administrator workspace is disposed', async () => {
    sectionState.settings = [
      { key: 'captcha.turnstile.site_key', value: 'reviewed-site-key', category: 'application', encrypted: false, configured: true, updatedAt: '2026-11-13T00:00:00Z' },
      { key: 'captcha.turnstile.enabled', value: 'true', category: 'application', encrypted: false, configured: true, updatedAt: '2026-11-13T00:00:00Z' },
    ]
    sectionState.perform.mockImplementation(async (task: () => Promise<void>) => {
      try { await task(); return true } catch { return false }
    })
    let finish!: () => void
    const update = vi.spyOn(api, 'updateAdminSetting').mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve }) as never)
    const wrapper = render()
    await flushPromises()
    await wrapper.get('button[form="admin-runtime-settings"]').trigger('click')
    await flushPromises()
    expect(update).toHaveBeenCalledExactlyOnceWith('captcha.turnstile.site_key', 'reviewed-site-key')
    wrapper.unmount()
    finish()
    await flushPromises()
    expect(update).toHaveBeenCalledOnce()
  })

  it.each(['activity', 'billing', 'profiles'] as const)('does not claim success after a %s section failure', async name => {
    saves[name].mockResolvedValue(false)
    const wrapper = render()
    await flushPromises()
    await wrapper.get('button[form="admin-runtime-settings"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain(t('adminSettings.partialSave'))
    expect(wrapper.text()).not.toContain(t('adminSettings.savedHint'))
    wrapper.unmount()
  })

  it('waits for all section results when one rejects', async () => {
    saves.billing.mockRejectedValue(new Error('save rejected'))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('button[form="admin-runtime-settings"]').trigger('click')
    await flushPromises()
    expect(saves.profiles).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain(t('adminSettings.partialSave'))
    wrapper.unmount()
  })

  it('does not restore an older activity snapshot after confirmed reward settings', async () => {
    reads.restoreRef.mockImplementation((_path: string, target: ShallowRef<ActivitySettings>) => { target.value = activity; return true })
    let resolve!: (value: ActivitySettings) => void
    reads.getActivity.mockReturnValue(new Promise(done => { resolve = done }))
    reads.saveActivity.mockResolvedValue({ ...activity, dailyRewardMinTxb: '3.00', dailyRewardMinTxbMinor: '300', dailyRewardMaxTxb: '4.00', dailyRewardMaxTxbMinor: '400' })
    const wrapper = render(true)
    await flushPromises()
    const fields = wrapper.findComponent(AdminActivitySettings).findAllComponents(TxbAmountField)
    fields.find(field => field.props('id') === 'daily-reward-minimum')!.vm.$emit('update:modelValue', '3.00')
    fields.find(field => field.props('id') === 'daily-reward-maximum')!.vm.$emit('update:modelValue', '4.00')
    await wrapper.get('button[form="admin-runtime-settings"]').trigger('click')
    await flushPromises()
    expect(reads.saveActivity).toHaveBeenCalledWith(expect.objectContaining({ dailyRewardMinTxb: '3.00', dailyRewardMaxTxb: '4.00' }))
    resolve(activity)
    await flushPromises()
    expect(wrapper.findComponent(AdminActivitySettings).props('settings')?.dailyRewardMinTxb).toBe('3.00')
    expect(fields.find(field => field.props('id') === 'daily-reward-maximum')!.props('modelValue')).toBe('4.00')
    expect(wrapper.text()).toContain(t('adminSettings.savedHint'))
    wrapper.unmount()
  })
})
