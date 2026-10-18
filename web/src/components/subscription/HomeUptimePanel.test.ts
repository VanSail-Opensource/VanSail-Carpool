import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'
import HomeUptimePanel from './HomeUptimePanel.vue'
import UptimeBar from './UptimeBar.vue'
import { subscriptionApi } from '@/api/subscription'

vi.mock('@/api/subscription', () => ({ subscriptionApi: {
  summary: vi.fn().mockResolvedValue({ activeCombo: false }),
} }))

describe('home subscription entrance', () => {
  it('navigates while loading without exposing a document URL to the WebView', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/home', component: { template: '<div />' } },
        { path: '/subscription', name: 'subscription', component: { template: '<div />' } },
      ],
    })
    await router.push('/home')
    const launchURL = window.location.href
    const wrapper = mount(HomeUptimePanel, { global: { plugins: [router] } })
    try {
      const entrance = wrapper.get('.home-uptime__entrance button')
      expect(entrance.attributes('type')).toBe('button')
      expect(wrapper.find('.home-uptime__entrance a').exists()).toBe(false)
      await entrance.trigger('click')
      await flushPromises()
      expect(router.currentRoute.value.name).toBe('subscription')
      expect(window.location.href).toBe(launchURL)
    } finally {
      wrapper.unmount()
    }
  })

  it('hides disabled monitoring while retaining subscription navigation', async () => {
    const now = new Date().toISOString()
    vi.mocked(subscriptionApi.summary).mockResolvedValue({ activeCombo: true, monitoringEnabled: false, from: now, to: now, state: 'unknown', segments: [], errorCode: '' })
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/home', component: { template: '<div />' } }, { path: '/subscription', name: 'subscription', component: { template: '<div />' } }] })
    await router.push('/home')
    const wrapper = mount(HomeUptimePanel, { global: { plugins: [router] } })
    try {
      await flushPromises()
      expect(wrapper.findComponent(UptimeBar).exists()).toBe(false)
      const entrance = wrapper.get('.home-uptime__entrance button')
      await entrance.trigger('click')
      await flushPromises()
      expect(router.currentRoute.value.name).toBe('subscription')
    } finally { wrapper.unmount() }
  })
})
