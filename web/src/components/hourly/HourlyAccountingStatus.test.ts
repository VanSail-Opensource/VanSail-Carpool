import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { HourlyAccountUsage } from '@/api/hourly'
import { setLocale, t } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { hourlyFixture } from '../../../fixtures/hourly-data'
import HourlyAccountingStatus from './HourlyAccountingStatus.vue'

type Accounting = NonNullable<HourlyAccountUsage['accounting']>
const empty: Accounting = {
  openReservedBytes: '0', recoveryUnknownBytes: '0', historicalRecoveryUnknownBytes: '0',
  historicalUnsealedReservedBytes: '0', historicalUnsealedCount: '0',
}
const usage = (accounting: Accounting | undefined, pending = true): HourlyAccountUsage => ({
  ...hourlyFixture(new URLSearchParams('first')).view.usage!, accounting, accountingPending: pending,
})
afterEach(() => setLocale('en'))

describe('hourly accounting evidence presentation', () => {
  for (const locale of ['en', 'zh-CN'] as const) {
    for (const state of [
      { name: 'ordinary open credit', accounting: { ...empty, openReservedBytes: '4096' }, historical: false, kind: 'open', amount: '4 KiB' },
      { name: 'current recovery unknown', accounting: { ...empty, recoveryUnknownBytes: '8192' }, historical: false, kind: 'recovery', amount: '8 KiB' },
      { name: 'historical recovery unknown', accounting: { ...empty, historicalRecoveryUnknownBytes: '16384' }, historical: true, kind: 'historical-recovery', amount: '16 KiB' },
      { name: 'historical unsealed proof', accounting: { ...empty, historicalUnsealedReservedBytes: '32768', historicalUnsealedCount: '2' }, historical: true, kind: 'historical-unsealed', amount: '32 KiB' },
      { name: 'historical unsealed proof with no remaining bytes', accounting: { ...empty, historicalUnsealedCount: '1' }, historical: true, kind: 'historical-unsealed', amount: '0 B' },
    ]) {
      it(`${locale}: distinguishes ${state.name}`, () => {
        setLocale(locale)
        const wrapper = mount(HourlyAccountingStatus, { props: { usage: usage(state.accounting), showHistorical: state.historical } })
        const messages = wrapper.findAll('[data-hourly-accounting-kind]')
        expect(messages).toHaveLength(1)
        expect(messages[0]!.attributes('data-hourly-accounting-kind')).toBe(state.kind)
        expect(messages[0]!.text()).toContain(state.amount)
        expect(wrapper.text()).not.toContain(t('hourly.accountingPending'))
        wrapper.unmount()
      })
    }
  }

  it('does not render a reconciliation warning for empty or fully admitted open credit', () => {
    for (const value of [null, usage(empty, false), usage(empty, true)]) {
      const wrapper = mount(HourlyAccountingStatus, { props: { usage: value } })
      expect(wrapper.find('[data-hourly-accounting]').exists()).toBe(false)
      wrapper.unmount()
    }
  })

  it('retains current and historical uncertainty together through a refresh', async () => {
    const bytes = '9007199254740993'
    const accounting = { ...empty, openReservedBytes: '4096', recoveryUnknownBytes: bytes, historicalRecoveryUnknownBytes: '16384' }
    const wrapper = mount(HourlyAccountingStatus, { props: { usage: usage(accounting), showHistorical: true } })
    expect(wrapper.findAll('[data-hourly-accounting-kind]')).toHaveLength(3)
    expect(wrapper.get('[data-hourly-accounting-kind="recovery"]').text()).toContain(formatBytes(bytes))
    await wrapper.setProps({ usage: usage({ ...accounting, openReservedBytes: '0' }) })
    expect(wrapper.find('[data-hourly-accounting-kind="open"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-hourly-accounting-kind]')).toHaveLength(2)
    wrapper.unmount()
  })

  it('ignores historical uncertainty by default while retaining current unknown amounts', async () => {
    const accounting = { ...empty, recoveryUnknownBytes: '8192', historicalRecoveryUnknownBytes: '16384', historicalUnsealedReservedBytes: '32768', historicalUnsealedCount: '2' }
    const wrapper = mount(HourlyAccountingStatus, { props: { usage: usage(accounting) } })
    expect(wrapper.findAll('[data-hourly-accounting-kind]')).toHaveLength(1)
    expect(wrapper.get('[data-hourly-accounting-kind="recovery"]').text()).toContain('8 KiB')
    await wrapper.setProps({ usage: usage({ ...accounting, recoveryUnknownBytes: '0' }) })
    expect(wrapper.find('[data-hourly-accounting]').exists()).toBe(false)
    expect(wrapper.props('usage')?.accountingPending).toBe(true)
    await wrapper.setProps({ showHistorical: true })
    expect(wrapper.findAll('[data-hourly-accounting-kind]')).toHaveLength(2)
    expect(wrapper.get('[data-hourly-accounting-kind="historical-recovery"]').text()).toContain('16 KiB')
    expect(wrapper.get('[data-hourly-accounting-kind="historical-unsealed"]').text()).toContain('32 KiB')
    wrapper.unmount()
  })

  it('retains a truthful compatibility notice when an older response has no breakdown', () => {
    const wrapper = mount(HourlyAccountingStatus, { props: { usage: usage(undefined) } })
    expect(wrapper.get('[data-hourly-accounting-kind="legacy"]').text()).toBe(t('hourly.accountingPending'))
    expect(wrapper.text()).not.toContain('still being reconciled')
    wrapper.unmount()
  })
})
