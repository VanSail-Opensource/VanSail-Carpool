import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { shallowRef } from 'vue'
import type { XNodeUserMetrics } from '@/api/xnodeMember'
import MemberLiveValues from './MemberLiveValues.vue'

vi.mock('@/composables/useMotionPreferences', () => ({ useMotionPreferences: () => ({ reducedMotion: shallowRef(true) }) }))
vi.mock('@/i18n', () => ({
  useI18n: () => ({ t: (key: string, values?: { value: string }) => values ? `${values.value}/s` : key }),
}))

const metrics: XNodeUserMetrics = {
  activityStatus: 'online', activityExpiresAt: null, fetchedAt: '2026-11-22T00:00:00Z',
  connections: 3, totalConnections: '9007199254740993', uploadBytesPerSecond: 1024,
  downloadBytesPerSecond: 2097152, incomplete: false, limited: false, reasons: [],
}

describe('member live values', () => {
  it('separates transfer rates from exact active and open counts', () => {
    const wrapper = mount(MemberLiveValues, { props: { metrics }, global: { stubs: { UIcon: true } } })
    expect(wrapper.get('[data-metric="download"] dd').text()).toBe('2MiB/s')
    expect(wrapper.get('[data-metric="upload"] dd').text()).toBe('1KiB/s')
    expect(wrapper.get('[data-metric="activeConnections"] dd').text()).toBe('3')
    expect(wrapper.get('[data-metric="openConnections"] dd').text()).toBe('9007199254740993')
    expect(wrapper.text()).toContain('memberNodes.activeConnectionsHint')
    expect(wrapper.find('[aria-live]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps missing totals unknown while retaining fresh zero rates', async () => {
    const wrapper = mount(MemberLiveValues, { props: { metrics: null }, global: { stubs: { UIcon: true } } })
    expect(wrapper.findAll('dd').every(value => value.text() === 'common.notAvailable')).toBe(true)
    await wrapper.setProps({ metrics: { ...metrics, totalConnections: undefined, downloadBytesPerSecond: 0 } })
    expect(wrapper.get('[data-metric="openConnections"] dd').text()).toBe('common.notAvailable')
    expect(wrapper.get('[data-metric="download"] dd').text()).toBe('0B/s')
    wrapper.unmount()
  })
})
