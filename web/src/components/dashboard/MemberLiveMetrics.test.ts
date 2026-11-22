import { shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { shallowRef } from 'vue'
import type { XNodeUserMetrics } from '@/api/xnodeMember'

import MemberLiveMetrics from './MemberLiveMetrics.vue'
import MemberLiveValues from './MemberLiveValues.vue'

const initial = (): { metrics: XNodeUserMetrics; deadline: number } => ({
  metrics: {
    activityStatus: 'online', activityExpiresAt: '2026-10-19T00:00:15Z', fetchedAt: '2026-10-19T00:00:00Z',
    connections: 1, totalConnections: '1', uploadBytesPerSecond: 0, downloadBytesPerSecond: 0,
    incomplete: false, limited: false, reasons: [],
  },
  deadline: performance.now() + 60_000,
})
const observation = shallowRef<{ metrics: XNodeUserMetrics; deadline: number } | null>(initial())
const error = shallowRef<string | null>(null)
const loading = shallowRef(false)
const refreshing = shallowRef(false)

vi.mock('@/components/subscription/useSubscriptionData', () => ({
  useSubscriptionData: () => ({ value: observation, error, loading, refreshing, refresh: vi.fn() }),
}))
vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }), t: (key: string) => key, getLocale: () => 'en' }))

describe('MemberLiveMetrics', () => {
  beforeEach(() => { observation.value = initial(); error.value = null; loading.value = false; refreshing.value = false })

  it('invalidates activity on focus or visibility change until a fresh response arrives', async () => {
    const wrapper = shallowMount(MemberLiveMetrics)
    expect(wrapper.emitted('activity')?.at(-1)).toEqual(['online'])
    window.dispatchEvent(new Event('focus'))
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('activity')?.at(-1)).toEqual(['unavailable'])
    observation.value = initial()
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('activity')?.at(-1)).toEqual(['online'])
    document.dispatchEvent(new Event('visibilitychange'))
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('activity')?.at(-1)).toEqual(['unavailable'])
    wrapper.unmount()
  })

  it('keeps the value surface stable through loading, expiry and read failure without announcing every poll', async () => {
    observation.value = null
    loading.value = true
    const wrapper = shallowMount(MemberLiveMetrics)
    const values = wrapper.getComponent(MemberLiveValues)
    expect(wrapper.get('[role="status"]').text()).toBe('common.loading')
    expect(values.props('metrics')).toBeNull()
    observation.value = initial()
    loading.value = false
    await wrapper.vm.$nextTick()
    expect(values.props('metrics')).not.toBeNull()
    expect(wrapper.get('section').attributes('aria-live')).toBeUndefined()
    observation.value = { ...initial(), deadline: performance.now() - 1 }
    await wrapper.vm.$nextTick()
    expect(wrapper.getComponent(MemberLiveValues).vm).toBe(values.vm)
    expect(values.props('metrics')).toBeNull()
    expect(wrapper.get('[role="status"]').text()).toBe('dashboard.activityUnavailable')
    observation.value = initial()
    error.value = 'xnode.loadFailed'
    await wrapper.vm.$nextTick()
    expect(values.props('metrics')).toBeNull()
    expect(wrapper.text()).toContain('xnode.loadFailed')
    wrapper.unmount()
  })

  it('preserves fresh zero and incomplete states and retires activity on unmount', async () => {
    observation.value = { ...initial(), metrics: { ...initial().metrics, activityStatus: 'offline', connections: 0, incomplete: true } }
    const wrapper = shallowMount(MemberLiveMetrics)
    expect(wrapper.getComponent(MemberLiveValues).props('metrics')?.connections).toBe(0)
    expect(wrapper.text()).toContain('memberNodes.incomplete')
    expect(wrapper.get('[role="status"]').text()).toBe('dashboard.offline')
    wrapper.unmount()
    expect(wrapper.emitted('activity')?.at(-1)).toEqual(['unavailable'])
  })
})
