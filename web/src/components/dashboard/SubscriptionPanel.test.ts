import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import SubscriptionPanel from './SubscriptionPanel.vue'

describe('subscription credential replacement', () => {
  it('retires old copy feedback and destructive consent when the URL changes', async () => {
    let finishCopy!: () => void
    const writeText = vi.fn(() => new Promise<void>(resolve => { finishCopy = resolve }))
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    const router = createRouter({ history: createMemoryHistory(), routes: [] })
    const wrapper = mount(SubscriptionPanel, { props: { subscriptionUrl: 'https://example.test/sub/old', revoking: false, revokeBlocked: false, openRevoke: true }, global: { plugins: [router], stubs: { ConfirmDialog: true } } })
    await wrapper.get('button[data-haptic="copy"]').trigger('click')
    expect(writeText).toHaveBeenCalledWith('https://example.test/sub/old')
    expect(wrapper.getComponent(ConfirmDialog).props('open')).toBe(true)
    await wrapper.setProps({ subscriptionUrl: 'https://example.test/sub/new' })
    finishCopy()
    await flushPromises()
    expect(wrapper.getComponent(ConfirmDialog).props('open')).toBe(false)
    expect(wrapper.get('button[data-haptic="copy"]').attributes('aria-label')).toBe('Copy subscription link')
    wrapper.getComponent(ConfirmDialog).vm.$emit('confirm')
    expect(wrapper.emitted('revoke')).toBeUndefined()
    expect(wrapper.emitted('revokeRequestConsumed')).toHaveLength(1)
    wrapper.unmount()
  })
})
