import { shallowMount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { shallowRef, type ShallowRef } from 'vue'
import type { HourlyView } from '@/api/hourly'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import type { useHourly } from '@/components/hourly/useHourly'
import { t } from '@/i18n'
import { hourlyFixture } from '../../../fixtures/hourly-data'
import SettingsHourlyControls from './SettingsHourlyControls.vue'
import SettingsSwitchRow from './SettingsSwitchRow.vue'

let state: ReturnType<typeof useHourly>
let snapshot: ShallowRef<HourlyView | null>
let busy: ShallowRef<boolean>
vi.mock('@/components/hourly/useHourly', () => ({ useHourly: () => state }))

function setup(query = 'active') {
  snapshot = shallowRef(hourlyFixture(new URLSearchParams(query)).view)
  busy = shallowRef(false)
  state = {
    value: snapshot, loading: shallowRef(false), liveReady: shallowRef(true), error: shallowRef(null), busy,
    submitting: shallowRef(false), actionError: shallowRef(null), refresh: vi.fn(), extra: vi.fn().mockResolvedValue(true),
  } as unknown as ReturnType<typeof useHourly>
  return shallowMount(SettingsHourlyControls, { global: { stubs: { InlineNotice: { template: '<div><slot /></div>' } } } })
}
beforeEach(() => vi.clearAllMocks())

describe('hourly automatic traffic settings', () => {
  it.each(['active', 'active&credit-exhausted'])('keeps the saved preference reachable in Settings for %s', query => {
    const wrapper = setup(query)
    expect(wrapper.getComponent(SettingsSwitchRow).props('enabled')).toBe(false)
    expect(wrapper.getComponent(SettingsSwitchRow).props('description')).toBe('1.25 TXB / GB')
    expect(state.extra).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('requires explicit price/expiry consent and retains the saved value after failed writes', async () => {
    const wrapper = setup()
    const row = wrapper.getComponent(SettingsSwitchRow)
    row.vm.$emit('update', true)
    await flushPromises()
    const dialog = wrapper.getComponent(ConfirmDialog)
    expect(dialog.props('open')).toBe(true)
    expect(dialog.props('description')).toContain('1.25 TXB')
    expect(dialog.props('description')).toContain('00:00 UTC')
    expect(row.props('enabled')).toBe(false)
    expect(state.extra).not.toHaveBeenCalled()
    vi.mocked(state.extra).mockImplementation(async () => { state.actionError.value = t('hourly.actionFailed'); return false })
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(state.extra).toHaveBeenCalledWith(true)
    expect(dialog.props('open')).toBe(true)
    expect(dialog.props('error')).toBe(t('hourly.actionFailed'))
    expect(row.props('enabled')).toBe(false)
    wrapper.unmount()
  })

  it('retires reviewed consent on a policy refresh and blocks stale submissions', async () => {
    const wrapper = setup()
    wrapper.getComponent(SettingsSwitchRow).vm.$emit('update', true)
    await flushPromises()
    snapshot.value = { ...snapshot.value!, policy: { ...snapshot.value!.policy, version: 'new-policy' } }
    await flushPromises()
    const dialog = wrapper.getComponent(ConfirmDialog)
    expect(dialog.props('open')).toBe(false)
    expect(wrapper.text()).toContain(t('hourly.conflict'))
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(state.extra).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('allows turning off an existing preference when new purchases become unavailable', async () => {
    const wrapper = setup('active&extra&no-extra')
    const row = wrapper.getComponent(SettingsSwitchRow)
    expect(row.props('enabled')).toBe(true)
    expect(row.props('disabled')).toBe(false)
    row.vm.$emit('update', false)
    await flushPromises()
    expect(state.extra).toHaveBeenCalledWith(false)
    expect(wrapper.getComponent(ConfirmDialog).props('open')).toBe(false)
    wrapper.unmount()
  })

  it('shows price review in Settings and keeps cached or unavailable controls disabled', async () => {
    const wrapper = setup('active&extra&price-rise')
    // The explicit price review remains a localized action, not a price-ceiling tag.
    expect(wrapper.find('[data-hourly-price-review]').exists()).toBe(true)
    busy.value = true
    await flushPromises()
    expect(wrapper.getComponent(SettingsSwitchRow).props('disabled')).toBe(true)
    wrapper.getComponent(SettingsSwitchRow).vm.$emit('update', false)
    await flushPromises()
    expect(state.extra).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('does not expose hourly preference changes for a legacy-blocked account', () => {
    const wrapper = setup('legacy')
    expect(wrapper.findComponent(SettingsSwitchRow).exists()).toBe(false)
    expect(state.extra).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
