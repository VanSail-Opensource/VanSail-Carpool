import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { preferencesApi } from '@/api/preferences'
import { usePreferencesStore } from '@/stores/preferences'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import ComingSoonLinks from './ComingSoonLinks.vue'

afterEach(() => vi.restoreAllMocks())

describe('ComingSoonLinks navigation', () => {
  it('keeps member-tool actions inside Vue Router history', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
    })
    await router.push('/home')
    await router.isReady()
    const pinia = createPinia()
    vi.spyOn(preferencesApi, 'get').mockResolvedValue({ paygEnabled: true, paygSquads: {}, activeCombo: true, showAroundTx: true, showActivity: false, includeNodePrices: true, showReferralUsername: true, notifications: { combos: true, traffic: true, money: true, activity: true, account: true } })
    usePreferencesStore(pinia).bind('around-member')
    await flushPromises()
    const wrapper = mount(ComingSoonLinks, { props: { hasValidCombo: true }, global: { plugins: [pinia, router] } })
    const launchURL = window.location.href
    const actions = wrapper.findAll('.home-around__link')
    const destinations = ['/knowledgebase', '/affiliates', '/community', '/emby']

    expect(actions).toHaveLength(destinations.length)
    for (const [index, destination] of destinations.entries()) {
      expect(actions[index].attributes('href')).toBeUndefined()
      await actions[index].trigger('click')
      await new Promise<void>((resolve) => setTimeout(resolve, 0))
      expect(router.currentRoute.value.path).toBe(destination)
      expect(window.location.href).toBe(launchURL)
      await router.push('/home')
    }
    wrapper.unmount()
  })

  it('keeps the Knowledgebase entrance without granting paid community tools', () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
    })
    const wrapper = mount(ComingSoonLinks, { props: { hasValidCombo: false }, global: { plugins: [createPinia(), router] } })

    expect(wrapper.find('.home-around').exists()).toBe(true)
    expect(wrapper.findAll('.home-around__link')).toHaveLength(1)
    expect(wrapper.text()).toContain('Knowledgebase')
    wrapper.unmount()
  })
})
