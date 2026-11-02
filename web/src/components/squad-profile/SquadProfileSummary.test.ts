import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import SquadProfileSummary from './SquadProfileSummary.vue'

describe('SquadProfileSummary', () => {
  it('projects international profile facts', () => {
    const wrapper = mount(SquadProfileSummary, {
      props: {
        profile: { type: 'international_network', portMbps: null, countryCode: 'SG', upstreamCarriers: ['Carrier A'] },
      },
    })
    expect(wrapper.text()).toContain('International Network')
    expect(wrapper.text()).toContain('Unlimited')
  })

  it('uses carrier marks without CT, CU, or CM prefixes for China routes', () => {
    const wrapper = mount(SquadProfileSummary, {
      props: {
        name: 'China route',
        profile: { type: 'china_optimized', ct: 'CN2', cu: 'CUG', cm: 'CMI', portMbps: null, countryCode: 'CN' },
        presentation: 'member',
      },
    })

    expect(wrapper.findAll('.carrier-logo')).toHaveLength(3)
    expect(wrapper.find('.carrier-logo--telecom').attributes('aria-label')).toBe('China Telecom')
    expect(wrapper.find('.carrier-logo--unicom').attributes('aria-label')).toBe('China Unicom')
    expect(wrapper.find('.carrier-logo--mobile').attributes('aria-label')).toBe('China Mobile')
    expect(wrapper.text()).toContain('China Optimized Unlimited')
    expect(wrapper.text()).not.toMatch(/\b(?:CT|CU|CM)\b/)
  })

  it('keeps member names free of the profile type icon and places prefix content first', () => {
    const wrapper = mount(SquadProfileSummary, {
      props: {
        name: 'Featured route',
        profile: { type: 'international_network', portMbps: null, countryCode: 'SG', upstreamCarriers: ['Transit'] },
        presentation: 'member',
      },
      slots: { namePrefix: '<span class="featured-marker">Featured</span>' },
    })

    const nameCopy = wrapper.get('.squad-profile-summary__name-copy').html()
    expect(wrapper.find('.squad-profile-summary__identity-icon').exists()).toBe(false)
    expect(nameCopy.indexOf('featured-marker')).toBeLessThan(nameCopy.indexOf('<strong'))
  })

  it('renders caller-owned state tags beside generated profile facts', () => {
    const wrapper = mount(SquadProfileSummary, {
      props: {
        profile: { type: 'broadband', isp: 'Harbor ISP', portMbps: 1_000, dynamic: false, location: 'Singapore' },
      },
      slots: { facts: '<span class="hidden-squad-tag">Hidden</span>' },
    })

    const facts = wrapper.get('.squad-profile-summary__facts')
    expect(facts.text()).toContain('Harbor ISP')
    expect(facts.text()).toContain('Singapore')
    expect(facts.get('.hidden-squad-tag').text()).toBe('Hidden')
  })

  it('keeps broadband location visible in member facts', () => {
    const wrapper = mount(SquadProfileSummary, {
      props: {
        name: 'Broadband route',
        profile: { type: 'broadband', isp: 'Harbor ISP', portMbps: 500, dynamic: false, location: 'Singapore' },
        presentation: 'member',
      },
    })

    expect(wrapper.get('.squad-profile-facts--member').text()).toContain('Singapore')
  })
})
