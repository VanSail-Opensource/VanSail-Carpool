import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it } from 'vitest'
import HomeSubscriptionEntrance from './HomeSubscriptionEntrance.vue'

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
    const wrapper = mount(HomeSubscriptionEntrance, { global: { plugins: [router] } })
    try {
      const entrance = wrapper.get('.subscription-entrance button')
      expect(entrance.attributes('type')).toBe('button')
      expect(wrapper.find('.subscription-entrance a').exists()).toBe(false)
      await entrance.trigger('click')
      await flushPromises()
      expect(router.currentRoute.value.name).toBe('subscription')
      expect(window.location.href).toBe(launchURL)
    } finally {
      wrapper.unmount()
    }
  })

})
