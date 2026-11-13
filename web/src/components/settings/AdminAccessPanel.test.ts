import { h, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminAccessLink } from '@/api/browserAccess'
import { t } from '@/i18n'

const mocks = vi.hoisted(() => ({ createLink: vi.fn(), copyText: vi.fn(), openExternalLink: vi.fn() }))
vi.mock('@/api/browserAccess', () => ({ browserAccessApi: mocks }))
vi.mock('@/utils/browserCompatibility', () => ({ copyText: mocks.copyText }))
vi.mock('@/utils/telegram', () => ({ openExternalLink: mocks.openExternalLink }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
const account = reactive<{ user: { id: string } | null; isAdmin: boolean }>({ user: { id: 'first' }, isAdmin: true })
vi.mock('@/stores/session', () => ({ useSessionStore: () => account }))
import AdminAccessPanel from './AdminAccessPanel.vue'

const stubs = {
  UAlert: { props: ['title'], template: '<p role="alert">{{ title }}</p>' },
  UInput: { props: ['modelValue'], render(this: { modelValue: string }) { return h('input', { value: this.modelValue }) } },
  UButton: { props: ['label', 'disabled'], render(this: { label: string; disabled: boolean }) { return h('button', { disabled: this.disabled }, this.label) } },
}
const link = (id: string): AdminAccessLink => ({ url: `https://panel.example/admin/access#token=${id}`, expiresAt: new Date(Date.now() + 60_000).toISOString() })
const mountPanel = () => mount(AdminAccessPanel, { global: { stubs } })
beforeEach(() => { vi.resetAllMocks(); account.user = { id: 'first' }; account.isAdmin = true; mocks.copyText.mockResolvedValue(true) })
afterEach(() => vi.restoreAllMocks())

describe('admin entrance link ownership and expiry', () => {
  it('ignores a retired owner response and clears a shown token on logout', async () => {
    const pending: Array<(value: AdminAccessLink) => void> = []
    mocks.createLink.mockImplementation(() => new Promise(resolve => { pending.push(resolve) }))
    const wrapper = mountPanel()
    account.user = { id: 'second' }
    pending[0]!(link('retired'))
    await flushPromises()
    expect(wrapper.find('input').exists()).toBe(false)
    pending[1]!(link('current'))
    await flushPromises()
    expect(wrapper.get('input').element.value).toContain('token=current')
    account.isAdmin = false; account.user = null
    await flushPromises()
    expect(wrapper.find('input').exists()).toBe(false)
    expect(mocks.createLink).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('does not copy a stale link when its expiry refresh fails', async () => {
    const old = link('old')
    mocks.createLink.mockResolvedValueOnce(old).mockRejectedValueOnce(new Error('constructed failure'))
    const wrapper = mountPanel()
    await flushPromises()
    vi.spyOn(Date, 'now').mockReturnValue(Date.parse(old.expiresAt) + 1)
    await wrapper.findAll('button').find(button => button.text() === t('browser.copyLink'))!.trigger('click')
    await flushPromises()
    expect(mocks.copyText).not.toHaveBeenCalled()
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.get('[role="alert"]').text()).toBe(t('browser.linkFailed'))
    wrapper.unmount()
  })
  it('does not carry delayed copy feedback into a replacement owner', async () => {
    mocks.createLink.mockImplementation(async () => link(account.user!.id))
    let complete!: (value: boolean) => void
    mocks.copyText.mockReturnValueOnce(new Promise(resolve => { complete = resolve }))
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === t('browser.copyLink'))!.trigger('click')
    account.user = { id: 'second' }
    await flushPromises()
    complete(true)
    await flushPromises()
    expect(wrapper.get('input').element.value).toContain('token=second')
    expect(wrapper.text()).not.toContain(t('browser.linkCopied'))
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
