import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { SquadProduct } from '@/api/types'
import SquadPricingTable from './SquadPricingTable.vue'
import SquadSelector from './SquadSelector.vue'

function squad(id: string, profile: SquadProduct['profile']): SquadProduct {
  return {
    id,
    remnaSquadUuid: `00000000-0000-4000-8000-${id.padStart(12, '0')}`,
    name: `Squad ${id}`,
    description: '',
    profile,
    price: { currency: 'TXB', minor: '100', display: '1.00 TXB' },
    visible: true,
    upstreamPresent: true,
    stockHeldByCurrentUser: false,
    activationRequired: false,
    accessibleNodes: [],
    createdAt: '2026-08-21T00:00:00Z',
    updatedAt: '2026-08-21T00:00:00Z',
  }
}

const international = squad('1', { type: 'international_network', portMbps: null, countryCode: 'SG', upstreamCarriers: ['Transit'] })
const broadband = squad('2', { type: 'broadband', isp: 'Harbor ISP', portMbps: 1_000, dynamic: false, location: 'Singapore' })
const optimized = squad('3', { type: 'china_optimized', ct: 'CN2', cu: '9929', cm: 'CMIN2', portMbps: 500, countryCode: 'JP' })

describe('SquadSelector', () => {
  it('renders one non-empty pricing table per profile type in the required order', () => {
    const wrapper = shallowMount(SquadSelector, {
      props: {
        squads: [optimized, broadband, international],
        selectedIds: [broadband.id],
        includedIds: [],
        featuredIds: [international.id],
        orderedIds: [international.id, broadband.id, optimized.id],
      },
    })

    const tables = wrapper.findAllComponents(SquadPricingTable)
    expect(tables.map(table => table.props('profileType'))).toEqual(['international_network', 'broadband', 'china_optimized'])
    expect(tables.map(table => table.props('squads').map((item: SquadProduct) => item.id))).toEqual([['1'], ['2'], ['3']])
    expect(tables[0]!.props('featuredIds')).toEqual([international.id])
  })

  it('does not render an empty table or an unconfigured legacy profile', () => {
    const unconfigured = squad('4', null)
    const wrapper = shallowMount(SquadSelector, {
      props: { squads: [international, unconfigured], selectedIds: [], includedIds: [], featuredIds: [], orderedIds: [international.id, unconfigured.id] },
    })

    expect(wrapper.findAllComponents(SquadPricingTable)).toHaveLength(1)
    expect(wrapper.text()).not.toContain(unconfigured.name)
  })

  it('omits combo-included squads from the optional squad step', () => {
    const wrapper = shallowMount(SquadSelector, {
      props: { squads: [international, broadband], selectedIds: [], includedIds: [international.id], featuredIds: [], orderedIds: [international.id, broadband.id] },
    })

    const tables = wrapper.findAllComponents(SquadPricingTable)
    expect(tables).toHaveLength(1)
    expect(tables[0]!.props('squads')).toEqual([broadband])
  })

  it('forwards table selection events', async () => {
    const wrapper = shallowMount(SquadSelector, {
      props: { squads: [international], selectedIds: [], includedIds: [], featuredIds: [], orderedIds: [international.id] },
    })
    const table = wrapper.getComponent(SquadPricingTable)

    table.vm.$emit('toggle', international.id)
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('toggle')).toEqual([[international.id]])
  })
})
