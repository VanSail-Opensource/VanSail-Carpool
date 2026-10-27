import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { SquadProduct } from '@/api/types'
import SquadPricingCard from './SquadPricingCard.vue'

const squad: SquadProduct = {
  id: 'squad', groupId: 'group', name: 'Pacific routes', description: '', price: { currency: 'TXB', minor: '125', display: '1.25 TXB' },
  visible: true, upstreamPresent: true, stockHeldByCurrentUser: false, activationRequired: false,
  accessibleNodes: [{ uuid: 'node', name: 'Internal Tokyo server', countryCode: 'JP', consumptionMultiplier: 1.5, providerName: null }],
  profile: null, createdAt: '2026-10-25T00:00:00Z', updatedAt: '2026-10-25T00:00:00Z',
}

describe('member squad profile cards', () => {
  it('hides combo squad prices but allows the separate PAYG price slot', () => {
    const props = { squad, selected: false, included: false, featured: false }
    const combo = mount(SquadPricingCard, { props })
    expect(combo.text()).not.toContain('1.25 TXB')
    const payg = mount(SquadPricingCard, { props: { ...props, informational: true }, slots: { price: 'PAYG rate' } })
    expect(payg.text()).toContain('PAYG rate')
  })
  it.each([false, true])('keeps carrier facts without exposing node names (PAYG=%s)', informational => {
    const wrapper = mount(SquadPricingCard, { props: {
      squad: { ...squad, profile: { type: 'international_network', portMbps: 1000, countryCode: 'JP', upstreamCarriers: ['KDDI', 'NTT'] } },
      selected: false, included: false, featured: false, informational,
    } })
    expect(wrapper.text()).toContain('Pacific routes')
    expect(wrapper.text()).toContain('KDDI, NTT')
    expect(wrapper.text()).toContain('Japan')
    expect(wrapper.text()).toContain('1000 Mbps')
    expect(wrapper.text()).not.toContain('Internal Tokyo server')
    expect(wrapper.text()).not.toContain('1.5x')
  })

  it.each([
    { profile: { type: 'china_optimized', portMbps: null, countryCode: 'JP', ct: 'CN2', cu: '9929', cm: 'CMIN2' }, facts: ['CN2', '9929', 'CMIN2', 'Japan', 'Unlimited'] },
    { profile: { type: 'broadband', portMbps: 500, isp: 'Harbor ISP', dynamic: false, location: 'Osaka' }, facts: ['Harbor ISP', 'Osaka', '500 Mbps'] },
  ] as const)('retains configured profile facts without accessible nodes', ({ profile, facts }) => {
    const wrapper = mount(SquadPricingCard, { props: {
      squad: { ...squad, profile, accessibleNodes: [] }, selected: false, included: false, featured: false,
    } })
    for (const fact of facts) expect(wrapper.text()).toContain(fact)
  })

  it('renders an unconfigured PAYG squad without inventing profile facts', () => {
    const wrapper = mount(SquadPricingCard, { props: { squad, selected: false, included: false, featured: false, informational: true } })
    expect(wrapper.text()).toContain('Pacific routes')
    expect(wrapper.find('.squad-profile-facts').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Internal Tokyo server')
  })
})
