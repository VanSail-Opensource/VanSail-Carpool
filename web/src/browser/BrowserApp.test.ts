/* eslint-disable vue/one-component-per-file */
import { defineComponent, h, reactive, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import BrowserApp from './BrowserApp.vue'

const identity = reactive({ user: { id: 'first-administrator' } })
const session = reactive({ expired: false, loading: false, error: null, config: null, bootstrap: vi.fn().mockResolvedValue(undefined), renew: vi.fn() })
const route = { name: 'browser-admin', fullPath: '/admin/xnode', meta: { console: true }, matched: [{}] }
vi.mock('@/stores/session', () => ({ useSessionStore: () => identity }))
vi.mock('./session', () => ({ useBrowserSession: () => session }))
vi.mock('vue-router', async importOriginal => ({ ...await importOriginal<typeof import('vue-router')>(), useRoute: () => route, useRouter: () => ({ replace: vi.fn() }) }))
vi.mock('./router', () => ({ browserRouteRecovery: { error: { value: null }, target: { value: null } } }))
vi.mock('./BrowserConsole.vue', () => ({ default: { name: 'BrowserConsole', template: '<section><slot /></section>' } }))

describe('browser console owner boundary', () => {
  it('preserves the same administrator draft through expiry and replaces it for another actor', async () => {
    const Editor = defineComponent({ name: 'AuditEditor', setup() { const draft = ref('unsaved'); return () => h('p', draft.value) } })
    const RouterView = defineComponent({ setup(_, { slots }) { return () => slots.default?.({ Component: Editor }) } })
    const wrapper = mount(BrowserApp, { global: { stubs: { App: { template: '<div><slot /></div>' }, Modal: true, RouterView } } })
    const original = wrapper.getComponent(Editor).vm
    session.expired = true
    await wrapper.vm.$nextTick()
    expect(wrapper.getComponent(Editor).vm).toBe(original)
    session.expired = false
    await wrapper.vm.$nextTick()
    expect(wrapper.getComponent(Editor).vm).toBe(original)
    identity.user.id = 'second-administrator'
    await wrapper.vm.$nextTick()
    expect(wrapper.getComponent(Editor).vm).not.toBe(original)
    wrapper.unmount()
  })
})
