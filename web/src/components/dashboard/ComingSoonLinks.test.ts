import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { usePreferencesStore } from '@/stores/preferences'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it } from 'vitest'

import ComingSoonLinks from './ComingSoonLinks.vue'

describe('ComingSoonLinks navigation', () => {
  it('keeps member-tool actions inside Vue Router history', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
    })
    await router.push('/home')
    await router.isReady()
    const pinia = createPinia()
    usePreferencesStore(pinia).value = { paygEnabled: true, paygSquads: {}, activeCombo: true, showAroundTx: true, showActivity: false, includeNodePrices: true, showReferralUsername: true, notifications: { combos: true, traffic: true, money: true, activity: true, account: true } }
    const wrapper = mount(ComingSoonLinks, { props: { hasValidCombo: true }, global: { plugins: [pinia, router] } })
    const launchURL = window.location.href
    const actions = wrapper.findAll('.home-around__link')
    const destinations = ['/affiliates', '/community', '/emby']

    expect(actions).toHaveLength(destinations.length)
    for (const [index, destination] of destinations.entries()) {
      expect(actions[index].attributes('href')).toBeUndefined()
      await actions[index].trigger('click')
      await new Promise<void>((resolve) => setTimeout(resolve, 0))
      expect(router.currentRoute.value.path).toBe(destination)
      expect(window.location.href).toBe(launchURL)
      await router.push('/home')
    }
  })

  it('does not render the Around TX entrance without a valid combo', () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
    })
    const wrapper = mount(ComingSoonLinks, { props: { hasValidCombo: false }, global: { plugins: [createPinia(), router] } })

    expect(wrapper.find('.home-around').exists()).toBe(false)
    wrapper.unmount()
  })
})
