import { shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { shallowRef } from 'vue'

import MemberLiveMetrics from './MemberLiveMetrics.vue'

const observation = shallowRef({
  metrics: {
    activityStatus: 'online', activityExpiresAt: '2026-10-19T00:00:15Z', fetchedAt: '2026-10-19T00:00:00Z',
    connections: 1, totalConnections: '1', uploadBytesPerSecond: 0, downloadBytesPerSecond: 0,
    incomplete: false, limited: false, reasons: [],
  },
  deadline: performance.now() + 60_000,
})
const error = shallowRef<string | null>(null)

vi.mock('@/components/subscription/useSubscriptionData', () => ({
  useSubscriptionData: () => ({ value: observation, error, loading: shallowRef(false), refresh: vi.fn() }),
}))
vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }), t: (key: string) => key, getLocale: () => 'en' }))

describe('MemberLiveMetrics', () => {
  it('invalidates activity on focus or visibility change until a fresh response arrives', async () => {
    const wrapper = shallowMount(MemberLiveMetrics)
    expect(wrapper.emitted('activity')?.at(-1)).toEqual(['online'])
    window.dispatchEvent(new Event('focus'))
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('activity')?.at(-1)).toEqual(['unavailable'])
    observation.value = { ...observation.value, deadline: performance.now() + 60_000 }
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('activity')?.at(-1)).toEqual(['online'])
    document.dispatchEvent(new Event('visibilitychange'))
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('activity')?.at(-1)).toEqual(['unavailable'])
    wrapper.unmount()
  })
})
