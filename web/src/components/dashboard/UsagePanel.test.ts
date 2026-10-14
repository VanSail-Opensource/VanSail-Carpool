import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import UsagePanel from './UsagePanel.vue'

describe('UsagePanel', () => {
  it('labels cached Remnawave statistics with an inline warning', () => {
    const wrapper = mount(UsagePanel, {
      props: {
        ratio: 0.25,
        stale: true,
        fetchedAt: '2026-08-07T00:00:00Z',
        statistics: {
          usedTrafficBytes: '26843545600',
          lifetimeTrafficBytes: '214748364800',
          trafficLimitBytes: '107374182400',
          onlineAt: null,
          lastTrafficResetAt: null,
          categories: [],
          sparklineData: ['0', '1073741824'],
          topNodes: [],
        },
        catalogNodes: [],
      },
    })
    expect(wrapper.text()).toContain('Last known data')
    expect(wrapper.text()).toContain('Remnawave is temporarily unavailable.')
    expect(wrapper.text()).toContain('OFFLINE')
    expect(wrapper.text()).toContain('No reset recorded')
    expect(wrapper.text()).toContain('25%')
    expect(wrapper.text()).toContain('25 GiB')
    expect(wrapper.text()).toContain('75 GiB')
    expect(wrapper.text()).not.toContain('of 100 GiB')
    wrapper.unmount()
  })

  it('marks a recent UTC online timestamp as online', () => {
    const wrapper = mount(UsagePanel, {
      props: {
        ratio: 0,
        stale: false,
        fetchedAt: '2026-08-07T00:00:00Z',
        statistics: {
          usedTrafficBytes: '0',
          lifetimeTrafficBytes: '0',
          trafficLimitBytes: '107374182400',
          onlineAt: new Date(Date.now() - 30_000).toISOString(),
          lastTrafficResetAt: '2026-08-06T12:34:00Z',
          categories: [],
          sparklineData: [],
          topNodes: [],
        },
        catalogNodes: [],
      },
    })
    expect(wrapper.text()).toContain('ONLINE')
    expect(wrapper.text()).toContain('Last traffic reset')
    expect(wrapper.text()).not.toContain('No reset recorded')
    wrapper.unmount()
  })
})
