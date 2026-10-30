import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { Session } from '@/api/types'
import { useSessionStore } from '@/stores/session'
import AppShell from './AppShell.vue'
import { desktopNavigationItems } from './navigation'

const api = vi.hoisted(() => ({ getCommunityAccess: vi.fn().mockResolvedValue({ activeCombo: true }) }))
vi.mock('@/api/client', () => ({ api }))

function session(role: Session['user']['role'] = 'user'): Session {
  return {
    authenticated: true,
    user: {
      id: 'user-1', telegramId: '42', firstName: 'Mira', lastName: 'Lin', telegramUsername: 'mira', displayName: '@mira', username: 'mira',
      role, onboardingState: 'complete', groupJoined: true, channelJoined: true,
      policyAcceptedAt: '2026-08-08T00:00:00Z', agreementRevision: 1, recoveryReason: '', createdAt: '2026-08-08T00:00:00Z', updatedAt: '2026-08-08T00:00:00Z',
    },
  }
}

describe('AppShell accessibility', () => {
  afterEach(() => {
    delete window.Telegram
    document.body.innerHTML = ''
  })

  it('restores focus after routing and drives Telegram BackButton', async () => {
    const show = vi.fn()
    const hide = vi.fn()
    const onClick = vi.fn()
    const offClick = vi.fn()
    const impactOccurred = vi.fn()
    window.Telegram = { WebApp: {
      version: '9.0', initData: '', initDataUnsafe: {}, colorScheme: 'dark', ready: vi.fn(), expand: vi.fn(), close: vi.fn(),
      openLink: vi.fn(), openTelegramLink: vi.fn(), openInvoice: vi.fn(),
      HapticFeedback: { impactOccurred, notificationOccurred: vi.fn() },
      BackButton: { isVisible: false, show, hide, onClick, offClick },
    } }
    const pinia = createPinia()
    useSessionStore(pinia).session = session()
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/home', component: { template: '<div />' } },
        { path: '/catalog', component: { template: '<div />' } },
        { path: '/balance', component: { template: '<div />' } },
        { path: '/activity', component: { template: '<div />' } },
        { path: '/emby', component: { template: '<div />' } },
        { path: '/settings/admin-access', component: { template: '<div />' } },
      ],
    })
    await router.push('/home')
    await router.isReady()
    const wrapper = mount(AppShell, { attachTo: document.body, global: { plugins: [pinia, router] }, slots: { default: '<h1>Content</h1>' } })

    expect(wrapper.get('main').attributes('tabindex')).toBe('-1')
    const scrollContainer = wrapper.get('.app-frame__content').element
    scrollContainer.scrollTop = 240
    await router.push('/catalog')
    await nextTick()
    expect(wrapper.get('.app-frame__content').element.scrollTop).toBe(0)
    expect(document.activeElement).toBe(wrapper.get('main').element)
    expect(show).toHaveBeenCalled()
    expect(onClick).toHaveBeenCalledOnce()
    onClick.mock.calls[0]?.[0]?.()
    expect(impactOccurred).toHaveBeenCalledWith('soft')

    const launchURL = window.location.href
    expect(wrapper.find('.side-rail').exists()).toBe(true)

    await router.push('/home')
    await nextTick()
    const bottomNavigationItems = wrapper.findAll('.bottom-nav__item')
    expect(bottomNavigationItems).toHaveLength(3)
    expect(bottomNavigationItems[0]?.attributes('data-state')).toBe('active')
    expect(wrapper.find('.bottom-nav__locale').exists()).toBe(false)
    expect(wrapper.find('.side-rail__footer').exists()).toBe(true)
    await router.push('/catalog')
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/catalog'))
    expect(window.location.href).toBe(launchURL)

    await router.push('/activity')
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/activity'))

    await router.push('/connections')
    await nextTick()
    expect(wrapper.findAll('.bottom-nav__item[data-state="active"]')).toHaveLength(0)

    wrapper.unmount()
    expect(offClick).toHaveBeenCalledOnce()
  })

  it('keeps administrators on the member navigation with console access in Settings', async () => {
    const pinia = createPinia()
    useSessionStore(pinia).session = session('admin')
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/home', component: { template: '<div />' } },
        { path: '/catalog', component: { template: '<div />' } },
        { path: '/activity', component: { template: '<div />' } },
        { path: '/settings/admin-access', component: { template: '<div />' } },
      ],
    })
    await router.push('/home')
    await router.isReady()
    const wrapper = mount(AppShell, { global: { plugins: [pinia, router] }, slots: { default: '<h1>Content</h1>' } })

    await nextTick()
    const bottomNavigationItems = wrapper.findAll('.bottom-nav__item')
    expect(bottomNavigationItems).toHaveLength(3)
    expect(wrapper.find('.bottom-nav__locale').exists()).toBe(false)
    expect(bottomNavigationItems.map(item => item.text()).join()).not.toContain('Admin access')
    expect(desktopNavigationItems(key => key, true).some(item => item.to === '/settings/admin-access')).toBe(false)

    await router.push('/settings/admin-access')
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/settings/admin-access'))
    expect(wrapper.findAll('.bottom-nav__item[data-state="active"]')).toHaveLength(0)

    wrapper.unmount()
  })

  it('keeps the desktop Home parent as a route link with expanded quick actions', () => {
    const home = desktopNavigationItems(key => key, false)[0]

    expect(home?.to).toBe('/home')
    expect(home?.type).not.toBe('trigger')
    expect(home?.defaultOpen).toBe(true)
    expect(home?.children).toHaveLength(3)
    expect(home?.children).toEqual(expect.arrayContaining([
      expect.objectContaining({ label: 'subscription.title', to: '/subscription' }),
      expect.objectContaining({ label: 'nav.revoke', to: { path: '/subscription', query: { revoke: '1' } } }),
    ]))
    expect(home?.children?.some(item => item.label === 'nav.addSquads')).toBe(false)
  })

  it('only includes the Around TX desktop entry for a valid combo', () => {
    const withoutCombo = desktopNavigationItems(key => key, false)
    const withCombo = desktopNavigationItems(key => key, false, true)

    expect(withoutCombo.find(item => item.label === 'nav.aroundTx')).toBeUndefined()
    expect(withCombo.find(item => item.label === 'nav.aroundTx')).toBeDefined()
  })
})
