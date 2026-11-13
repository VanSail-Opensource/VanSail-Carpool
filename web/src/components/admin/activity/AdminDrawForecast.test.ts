import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { featuresApi, type LuckyDrawForecast } from '@/api/features'
import { t } from '@/i18n'
import AdminDrawForecast from './AdminDrawForecast.vue'

function forecast(incomeMinor: string): LuckyDrawForecast {
  return { entries: 1, incomeMinor, expectedExpenseMinMinor: '0', expectedExpenseMaxMinor: '0',
    possibleExpenseMinMinor: '0', possibleExpenseMaxMinor: '0', breakEvenMinMinor: null,
    breakEvenMaxMinor: null, averageBalanceMinor: null, multiplierUnavailable: true, couponOneTermOnly: false }
}
afterEach(() => vi.restoreAllMocks())

describe('selected draw forecast', () => {
  it.each(['success', 'failure'])('retires the previous draw forecast %s after selection changes', async outcome => {
    let resolve!: (value: LuckyDrawForecast) => void
    let reject!: (error: Error) => void
    vi.spyOn(featuresApi, 'getAdminLuckyDrawForecast')
      .mockReturnValueOnce(new Promise((done, fail) => { resolve = done; reject = fail }))
      .mockResolvedValue(forecast('20000'))
    const wrapper = mount(AdminDrawForecast, { props: { drawId: 'old-draw' } })
    await wrapper.setProps({ drawId: 'current-draw' })
    await flushPromises()
    expect(wrapper.text()).toContain('200.00 TXB')
    if (outcome === 'success') resolve(forecast('10000'))
    else reject(new Error('retired forecast'))
    await flushPromises()
    expect(wrapper.text()).toContain('200.00 TXB')
    expect(wrapper.text()).not.toContain('100.00 TXB')
    expect(wrapper.text()).not.toContain(t('adminDrawRedesign.forecastError'))
    wrapper.unmount()
  })

  it('exposes an explicit retry after a failed read', async () => {
    const read = vi.spyOn(featuresApi, 'getAdminLuckyDrawForecast').mockRejectedValueOnce(new Error('unavailable')).mockResolvedValue(forecast('20000'))
    const wrapper = mount(AdminDrawForecast, { props: { drawId: 'draw' } })
    await flushPromises()
    expect(wrapper.text()).toContain(t('adminDrawRedesign.forecastError'))
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(read).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('200.00 TXB')
    wrapper.unmount()
  })
})
