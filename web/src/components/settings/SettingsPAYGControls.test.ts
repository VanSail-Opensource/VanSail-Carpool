import { shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive, shallowRef } from 'vue'

import SettingsPAYGControls from './SettingsPAYGControls.vue'
import SettingsSwitchRow from './SettingsSwitchRow.vue'

const data = shallowRef<{ balance: { minor: string }; squads: { squad: { billingMode: string; groupId: string; name: string } }[] } | null>(null)
const error = shallowRef<string | null>(null)
const loading = shallowRef(false)
const refreshing = shallowRef(false)
const liveReady = shallowRef(true)
const preferences = reactive({ value: { paygEnabled: true, paygSquads: { 'squad-1': false } }, saving: false, blocked: false, pendingKey: null, save: vi.fn() })

vi.mock('@/components/subscription/useSubscriptionData', () => ({
  useSubscriptionData: () => ({ value: data, error, loading, refreshing, liveReady, refresh: vi.fn() }),
}))
vi.mock('@/stores/preferences', () => ({ usePreferencesStore: () => preferences }))

function mountControls() {
  return shallowMount(SettingsPAYGControls, { global: { mocks: { $t: (key: string) => key }, stubs: { InlineNotice: { template: '<div><slot /></div>' } } } })
}

describe('SettingsPAYGControls', () => {
  beforeEach(() => {
    data.value = { balance: { minor: '1' }, squads: [{ squad: { billingMode: 'payg', groupId: 'squad-1', name: 'Tokyo' } }] }
    error.value = null
    loading.value = false
    refreshing.value = false
    liveReady.value = true
    preferences.value.paygEnabled = true
    preferences.value.paygSquads['squad-1'] = false
    preferences.saving = false
    preferences.blocked = false
    preferences.save.mockClear()
  })

  it('renders cached choices without allowing changes before fresh eligibility arrives', async () => {
    liveReady.value = false
    const wrapper = mountControls()
    expect(wrapper.findAllComponents(SettingsSwitchRow).every(row => row.props('disabled'))).toBe(true)
    liveReady.value = true
    await wrapper.vm.$nextTick()
    expect(wrapper.findAllComponents(SettingsSwitchRow).every(row => !row.props('disabled'))).toBe(true)
    preferences.blocked = true
    await wrapper.vm.$nextTick()
    expect(wrapper.findAllComponents(SettingsSwitchRow).every(row => row.props('disabled'))).toBe(true)
  })

  it('keeps the saved switches visible but disables them at zero or negative balance', async () => {
    const wrapper = mountControls()
    expect(wrapper.findAllComponents(SettingsSwitchRow).map(row => row.props('disabled'))).toEqual([false, false])
    data.value = { ...data.value!, balance: { minor: '0' } }
    await wrapper.vm.$nextTick()
    const rows = wrapper.findAllComponents(SettingsSwitchRow)
    expect(rows.map(row => row.props('disabled'))).toEqual([true, true])
    expect(rows.map(row => row.props('enabled'))).toEqual([true, false])
    expect(wrapper.text()).toContain('settings.payg.balanceRequired')
    data.value = { ...data.value!, balance: { minor: '-1' } }
    await wrapper.vm.$nextTick()
    expect(wrapper.findAllComponents(SettingsSwitchRow).every(row => row.props('disabled'))).toBe(true)
    expect(preferences.save).not.toHaveBeenCalled()
  })

  it('restores controls after a positive balance and disables them on load failure', async () => {
    data.value = { ...data.value!, balance: { minor: '0' } }
    const wrapper = mountControls()
    data.value = { ...data.value!, balance: { minor: '99999999999999999999' } }
    await wrapper.vm.$nextTick()
    expect(wrapper.findAllComponents(SettingsSwitchRow).map(row => row.props('disabled'))).toEqual([false, false])
    error.value = 'load failed'
    await wrapper.vm.$nextTick()
    expect(wrapper.findAllComponents(SettingsSwitchRow).every(row => row.props('disabled'))).toBe(true)
  })
})
