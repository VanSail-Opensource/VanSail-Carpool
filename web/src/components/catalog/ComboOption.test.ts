import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'

import type { Combo } from '@/api/types'
import { resetDisplayCurrency, setDisplayCurrency } from '@/utils/displayCurrency'
import ComboOption from './ComboOption.vue'

const combo: Combo = {
  id: 'combo-1',
  name: 'Weekend North',
  description: 'A compact monthly plan for a small device set.',
  price: { currency: 'TXB', minor: '1880', display: '18.80 TXB' },
  validityDays: 30,
  trafficLimitBytes: '107374182400',
  resetStrategy: 'MONTH_ROLLING',
  active: true,
  includedSquads: [],
  createdAt: '2026-08-07T00:00:00Z',
  updatedAt: '2026-08-07T00:00:00Z',
}

const comboWithIncludedSquad: Combo = {
  ...combo,
  includedSquads: [{
    id: 'squad-1',
    groupId: '00000000-0000-4000-8000-000000000001',
    name: 'Private included squad',
    description: 'Private included squad detail',
    profile: null,
    price: { currency: 'TXB', minor: '0', display: '0.00 TXB' },
    visible: true,
    upstreamPresent: true,
    stockHeldByCurrentUser: false,
    activationRequired: false,
    accessibleNodes: [{
      uuid: '00000000-0000-4000-8000-000000000002',
      name: 'Private included node',
      countryCode: 'US',
      providerName: 'Private provider',
      consumptionMultiplier: 1,
    }],
    createdAt: '2026-08-07T00:00:00Z',
    updatedAt: '2026-08-07T00:00:00Z',
  }],
}

describe('ComboOption', () => {
  afterEach(() => resetDisplayCurrency())

  it('renders server-formatted price and entitlement values', () => {
    const wrapper = mount(ComboOption, { props: { combo, selected: false } })
    expect(wrapper.text()).toContain('18.80 TXB')
    expect(wrapper.text()).toContain('100 GiB/month')
    expect(wrapper.text()).toContain('per 30 days')
    expect(wrapper.text()).toContain('Above 0.00% remaining')
  })

  it('uses the configured display currency directly', () => {
    setDisplayCurrency({ currency: 'USD', rates: { cnyTxbPerUnit: null, usdTxbPerUnit: '2.5' } })

    const wrapper = mount(ComboOption, { props: { combo, selected: false } })

    expect(wrapper.text()).toContain('$7.52')
    expect(wrapper.text()).not.toContain('18.80 TXB')
  })

  it('does not display included squad detail', () => {
    const wrapper = mount(ComboOption, { props: { combo: comboWithIncludedSquad, selected: false } })

    expect(wrapper.text()).not.toContain('Private included squad')
    expect(wrapper.text()).not.toContain('Private included node')
    expect(wrapper.text()).not.toContain('Private provider')
  })

  it('emits the stable combo id on selection', async () => {
    const wrapper = mount(ComboOption, { props: { combo, selected: false } })
    await wrapper.trigger('click')
    expect(wrapper.emitted('select')).toEqual([['combo-1']])
  })
})
