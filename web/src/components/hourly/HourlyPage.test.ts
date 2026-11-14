import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { h, shallowRef, type ShallowRef } from 'vue'
import { hourlyFixture } from '../../../fixtures/hourly-data'
import { t } from '@/i18n'
import { resetDisplayCurrency, setDisplayCurrency } from '@/utils/displayCurrency'
import HourlyPage from './HourlyPage.vue'
import HourlySummary from './HourlySummary.vue'
import HourlySquadCard from './HourlySquadCard.vue'
import type { useHourly } from './useHourly'
import type { OperationReceipt } from '@/api/types'
import type { HourlyView } from '@/api/hourly'

let state: ReturnType<typeof useHourly>
let snapshot: ShallowRef<HourlyView | null>
vi.mock('./useHourly', () => ({ useHourly: () => state }))
const Button = { props: ['label', 'disabled'], emits: ['click'], render(this: { label: string; disabled: boolean; $emit: (event: string) => void }) { return h('button', { disabled: this.disabled, onClick: () => this.$emit('click') }, this.label) } }
const Confirm = { props: ['open', 'description', 'confirmLabel'], emits: ['confirm'], render(this: { open: boolean; description: string; confirmLabel: string; $emit: (event: string) => void }) { return this.open ? h('section', { 'data-confirm': '' }, [h('p', this.description), h('button', { onClick: () => this.$emit('confirm') }, this.confirmLabel)]) : null } }
const Code = { props: ['open'], emits: ['submit'], render(this: { open: boolean; $emit: (event: string, value: string) => void }) { return this.open ? h('button', { 'data-code': '', onClick: () => this.$emit('submit', 'secret-code') }) : null } }
function setup(query = '') {
  const view = hourlyFixture(new URLSearchParams(query)).view
  snapshot = shallowRef(view)
  state = {
    value: snapshot, loading: shallowRef(false), liveReady: shallowRef(true), refreshing: shallowRef(false), error: shallowRef(null), hasData: shallowRef(true),
    busy: shallowRef(false), submitting: shallowRef(false), actionError: shallowRef(null), refresh: vi.fn(),
    activate: vi.fn().mockResolvedValue(true), disable: vi.fn().mockResolvedValue(true), extra: vi.fn().mockResolvedValue(true),
    operation: { receipt: shallowRef(query.includes('failed') ? { id: 'failed', kind: 'hourly_access', status: 'failed', errorCode: 'HOURLY_LEGACY_ACTIVE', createdAt: '', updatedAt: '', completedAt: null } satisfies OperationReceipt : null), polling: shallowRef(false), checking: shallowRef(false), error: shallowRef(null), terminal: shallowRef(false), track: vi.fn(), refresh: vi.fn(), reset: vi.fn() },
  } as unknown as ReturnType<typeof useHourly>
  return mount(HourlyPage, { global: { stubs: { UButton: Button, ConfirmDialog: Confirm, SquadActivationDialog: Code, USwitch: true, StatusBadge: true, HourlyAllowanceGraph: { template: '<div />' }, SquadPricingCard: { template: '<article><slot name="status" /><slot name="price" /><slot /></article>' }, SquadProfileSummary: true, InlineNotice: { template: '<div><slot /></div>' }, OperationStatusNotice: { props: ['message'], template: '<p data-operation>{{ message }}</p>' } } } })
}
beforeEach(() => vi.clearAllMocks())
afterEach(() => resetDisplayCurrency())

describe('hourly consent and access controls', () => {
  it('does not show the member summary for historical-only accounting obligations', async () => {
    const wrapper = setup('first&historical-unknown&historical-unsealed')
    expect(state.value.value?.usage?.accountingPending).toBe(true)
    expect(wrapper.findComponent(HourlySummary).exists()).toBe(false)
    const current = state.value.value!
    snapshot.value = { ...current, usage: { ...current.usage!, freeReservedBytes: '8192', accounting: { ...current.usage!.accounting!, recoveryUnknownBytes: '8192' } } }
    await flushPromises()
    expect(wrapper.findComponent(HourlySummary).exists()).toBe(true)
    expect(wrapper.find('[data-hourly-accounting]').exists()).toBe(false)
    expect(state.value.value?.usage?.accounting?.recoveryUnknownBytes).toBe('8192')
    expect(wrapper.find('[data-hourly-accounting-kind="historical-recovery"]').exists()).toBe(false)
    expect(wrapper.find('[data-hourly-accounting-kind="historical-unsealed"]').exists()).toBe(false)
    expect(state.activate).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('retires reviewed extra-traffic terms when a refresh changes the policy', async () => {
    const wrapper = setup('active')
    wrapper.findComponent(HourlySummary).vm.$emit('extra', true)
    await flushPromises()
    expect(wrapper.get('[data-confirm]').text()).toContain('1.25 TXB')
    const current = state.value.value!
    snapshot.value = { ...current, policy: { version: 'updated-policy', policy: { ...current.policy.policy, extraGBPriceTXBMinor: '250' } } }
    await flushPromises()
    expect(wrapper.find('[data-confirm]').exists()).toBe(false)
    expect(wrapper.text()).toContain(t('hourly.conflict'))
    expect(state.extra).not.toHaveBeenCalled()
    wrapper.findComponent(HourlySummary).vm.$emit('extra', true)
    await flushPromises()
    expect(wrapper.get('[data-confirm]').text()).toContain('2.50 TXB')
    await wrapper.get('[data-confirm] button').trigger('click')
    expect(state.extra).toHaveBeenCalledOnce()
    expect(state.extra).toHaveBeenCalledWith(true)
    wrapper.unmount()
  })
  it('retires an activation code dialog when its offer revision changes', async () => {
    const wrapper = setup('activation')
    wrapper.findComponent(HourlySquadCard).vm.$emit('activate')
    await flushPromises()
    expect(wrapper.find('[data-code]').exists()).toBe(true)
    const current = state.value.value!
    snapshot.value = { ...current, squads: current.squads.map((item, index) => index === 0 ? { ...item, offer: { ...item.offer, revision: '2' } } : item) }
    await flushPromises()
    expect(wrapper.find('[data-code]').exists()).toBe(false)
    expect(state.activate).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('retires disable consent when the paid enrollment is replaced', async () => {
    const wrapper = setup('active')
    wrapper.findComponent(HourlySquadCard).vm.$emit('disable')
    await flushPromises()
    const current = state.value.value!
    snapshot.value = { ...current, squads: current.squads.map((item, index) => index === 0 ? { ...item, enrollment: { ...item.enrollment!, id: 'replacement-enrollment' } } : item) }
    await flushPromises()
    expect(wrapper.find('[data-confirm]').exists()).toBe(false)
    expect(state.disable).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('shows a terminal refusal instead of promising an application that cannot complete', () => {
    const wrapper = setup('failed')
    expect(wrapper.get('[data-operation]').text()).toBe(t('hourly.result.HOURLY_LEGACY_ACTIVE'))
    expect(wrapper.get('[data-operation]').text()).not.toContain(t('hourly.applicationPending'))
    wrapper.unmount()
  })
  it('does not purchase extras before explicit price and expiry consent', async () => {
    const wrapper = setup()
    wrapper.findComponent(HourlySummary).vm.$emit('extra', true)
    await flushPromises()
    expect(state.extra).not.toHaveBeenCalled()
    expect(wrapper.get('[data-confirm]').text()).toContain('1.25 TXB')
    expect(wrapper.get('[data-confirm]').text()).toContain('1,000,000,000')
    expect(wrapper.get('[data-confirm]').text()).toContain('00:00 UTC')
    await wrapper.get('[data-confirm] button').trigger('click')
    expect(state.extra).toHaveBeenCalledWith(true)
    wrapper.unmount()
  })
  it('shows the native approved price even when the member prefers another currency', async () => {
    setDisplayCurrency({ currency: 'USD', rates: { usdTxbPerUnit: '10', cnyTxbPerUnit: '2' } })
    const wrapper = setup('active')
    expect(wrapper.text()).not.toContain('Your approved price ceiling')
    expect(wrapper.find('u-switch-stub').exists()).toBe(false)
    wrapper.findComponent(HourlySummary).vm.$emit('extra', true)
    await flushPromises()
    expect(wrapper.get('[data-confirm]').text()).toContain('1.25 TXB')
    expect(wrapper.get('[data-confirm]').text()).not.toContain('0.12 TXB')
    wrapper.unmount()
  })

  it('warns about the paid hour and exact cooldown before voluntary disable', async () => {
    const wrapper = setup('active')
    wrapper.findComponent(HourlySquadCard).vm.$emit('disable')
    await flushPromises()
    expect(state.disable).not.toHaveBeenCalled()
    expect(wrapper.get('[data-confirm]').text()).toContain('24 hours')
    expect(wrapper.get('[data-confirm]').text()).toContain('not refunded')
    await wrapper.get('[data-confirm] button').trigger('click')
    expect(state.disable).toHaveBeenCalledWith(state.value.value!.squads[0])
    wrapper.unmount()
  })
  it('blocks legacy accounts while leaving the grandfathered renewal explanation visible', () => {
    const wrapper = setup('legacy')
    expect(wrapper.text()).toContain(t('hourly.legacyBlocked'))
    expect(wrapper.findAll('button').filter(button => button.attributes('aria-label') === t('hourly.activate')).every(button => button.attributes('disabled') !== undefined)).toBe(true)
    expect(state.activate).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('collects required activation codes without submitting an unprotected activation', async () => {
    const wrapper = setup('activation')
    wrapper.findComponent(HourlySquadCard).vm.$emit('activate')
    await flushPromises()
    expect(state.activate).not.toHaveBeenCalled()
    await wrapper.get('[data-code]').trigger('click')
    expect(state.activate).toHaveBeenCalledWith(state.value.value!.squads[0], 'secret-code')
    wrapper.unmount()
  })
})
