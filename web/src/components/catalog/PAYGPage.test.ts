import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia } from 'pinia'
import { paygApi, type PAYGView } from '@/api/payg'
import PAYGPage from './PAYGPage.vue'
import PAYGSummary from './PAYGSummary.vue'

const view: PAYGView = {
  balance: { currency: 'TXB', minor: '2500', display: '25.00 TXB' },
  asOf: '2026-10-25T12:00:00Z', windowStart: '2026-10-22T12:00:00Z',
  squads: [], trafficBytes: '9663676416', spent: { currency: 'TXB', minor: '1500', display: '15.00 TXB' },
}

afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

function mountPage(entranceReady = true) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  return mount(PAYGPage, { props: { entranceReady }, global: { plugins: [router, createPinia()] } })
}

describe('PAYG live presentation', () => {
  it('keeps resolved totals paused until its scene releases the entrance', async () => {
    vi.spyOn(paygApi, 'get').mockResolvedValue(view)
    const wrapper = mountPage(false)
    await flushPromises()
    expect(wrapper.text()).toContain('15.00 TXB')
    expect(wrapper.get('.payg-summary__window').classes()).toContain('payg-summary__window--enter')
    expect(wrapper.get('.payg-summary__window').classes()).toContain('payg-summary__window--waiting')
    await wrapper.setProps({ entranceReady: true })
    expect(wrapper.get('.payg-summary__window').classes()).not.toContain('payg-summary__window--waiting')
    wrapper.unmount()
  })
  it('renders loading and empty states with one reporting window and no recharge action', async () => {
    vi.spyOn(paygApi, 'get').mockResolvedValue(view)
    const wrapper = mountPage()
    expect(wrapper.find('[aria-busy=true]').exists()).toBe(true)
    await flushPromises()
    expect(wrapper.text()).toContain('25.00 TXB')
    expect(wrapper.text()).toContain('9 GiB')
    expect(wrapper.text()).toContain('15.00 TXB')
    expect(wrapper.find('.payg-page__empty').exists()).toBe(true)
    expect(wrapper.find('a[href="/settings"]').exists()).toBe(false)
    expect(wrapper.text().match(/Last 72 hours/g)).toHaveLength(1)
    expect(wrapper.find('a[href="/home?topUp=1"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('retains wallet totals when a background refresh fails', async () => {
    vi.useFakeTimers()
    const fetcher = vi.spyOn(paygApi, 'get').mockResolvedValue(view)
    const wrapper = mountPage()
    await flushPromises()
    fetcher.mockRejectedValue(new Error('offline'))
    await vi.advanceTimersByTimeAsync(15_000)
    await flushPromises()
    expect(wrapper.find('.payg-page__error').exists()).toBe(true)
    expect(wrapper.findComponent(PAYGSummary).props('data')).toEqual(view)
    expect(wrapper.text()).toContain('25.00 TXB')
    expect(wrapper.text()).toContain('15.00 TXB')
    wrapper.unmount()
  })
})
