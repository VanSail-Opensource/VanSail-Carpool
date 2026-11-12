import { defineComponent, h, shallowRef } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { t } from '@/i18n'

const saves = vi.hoisted(() => ({ emby: vi.fn(), activity: vi.fn(), billing: vi.fn(), profiles: vi.fn() }))
vi.mock('@/composables/useAdminSection', () => ({ useAdminSection: () => ({
  items: shallowRef([]), loading: shallowRef(false), busy: shallowRef(false), error: shallowRef(null),
  load: vi.fn(), perform: vi.fn().mockResolvedValue(true),
}) }))
vi.mock('@/api/features', () => ({ featuresApi: { getAdminActivitySettings: vi.fn().mockResolvedValue({ timezone: 'UTC', dailyRewardMinTxb: '0.00', dailyRewardMaxTxb: '0.00', groupMessageThreshold: 0, groupMessageRewardTxb: '0.00' }) } }))
import AdminSettingsPanel from './AdminSettingsPanel.vue'

function section(save: () => Promise<boolean>, method = 'save') {
  return defineComponent({ setup(_, { expose }) { expose({ [method]: save, loading: false }); return () => h('div') } })
}
function render() {
  return mount(AdminSettingsPanel, { global: { stubs: {
    AdminSectionState: { template: '<div><slot /></div>' },
    AdminEmbyConnection: section(saves.emby), AdminActivitySettings: section(saves.activity),
    AdminBillingAmountLimits: section(saves.billing), AdminPaymentProfiles: section(saves.profiles, 'saveAll'),
    InlineNotice: { props: ['title'], template: '<div>{{ title }}<slot /></div>' },
  } } })
}
beforeEach(() => { Object.values(saves).forEach(save => save.mockReset().mockResolvedValue(true)) })

describe('aggregate settings persistence', () => {
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
})
