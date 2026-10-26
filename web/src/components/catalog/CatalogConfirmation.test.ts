import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it } from 'vitest'

import type { Purchase } from '@/api/types'
import CatalogConfirmation from './CatalogConfirmation.vue'

const purchase = {
  id: 'purchase-1',
  comboId: 'combo-1',
  comboName: 'Combo',
  price: { currency: 'TXB', minor: '900', display: '9.00 TXB' },
  grossPrice: { currency: 'TXB', minor: '1000', display: '10.00 TXB' },
  couponDiscount: { currency: 'TXB', minor: '100', display: '1.00 TXB' },
  couponGrantId: null,
  validFrom: '2026-08-13T00:00:00Z',
  validUntil: '2026-09-12T00:00:00Z',
  status: 'activating',
  autoRenewEnabled: true,
  trafficLimitBytes: '1073741824',
  resetStrategy: 'MONTH_ROLLING',
  squadUuids: [],
  createdAt: '2026-08-13T00:00:00Z',
  updatedAt: '2026-08-13T00:00:00Z',
} as Purchase

async function mountConfirmation() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/catalog', component: { template: '<div />' } }],
  })
  await router.push('/catalog')
  await router.isReady()
  return { router, wrapper: mount(CatalogConfirmation, { global: { plugins: [router] }, props: { purchase } }) }
}

describe('CatalogConfirmation', () => {
  it('renders the server-confirmed purchase summary', async () => {
    const { wrapper } = await mountConfirmation()

    expect(wrapper.get('[data-test="catalog-confirmation"]').text()).toContain('Combo')
    expect(wrapper.text()).toContain('9.00 TXB')
    expect(wrapper.text()).toContain('1.00 TXB')
    expect(wrapper.text()).toContain('Activating')
    expect(wrapper.text()).toContain('1 GiB')

    wrapper.unmount()
  })

  it('emits Home navigation when the primary action is clicked', async () => {
    const { wrapper } = await mountConfirmation()

    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('home')).toHaveLength(1)
    wrapper.unmount()
  })
})
