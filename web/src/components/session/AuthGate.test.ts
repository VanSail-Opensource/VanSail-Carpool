import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { h, type Slots } from 'vue'
import { setLocale, t } from '@/i18n'
import AuthGate from './AuthGate.vue'

vi.mock('@/utils/telegram', () => ({ isTelegramWebAppDetected: () => true }))
vi.mock('motion-v', async () => {
  const { h } = await import('vue')
  return { motion: { main: { setup: (_: unknown, { slots }: { slots: Slots }) => () => h('main', null, slots.default?.()) } } }
})

afterEach(() => setLocale('en'))

describe('verified cleanup recovery heading', () => {
  it.each(['en', 'zh-CN'] as const)('uses an accurate %s title and keeps retry available', async locale => {
    setLocale(locale)
    const view = mount(AuthGate, {
      props: { code: 'ACCOUNT_RECOVERY_PENDING', message: t('errors.api.ACCOUNT_RECOVERY_PENDING') },
      global: { mocks: { $t: t }, stubs: {
        LanguageControl: true,
        UButton: { props: ['label'], emits: ['click'], setup: (props, { emit }) => () => h('button', { onClick: () => emit('click') }, props.label) },
      } },
    })
    expect(view.get('h1').text()).toBe(t('auth.accountRecoveryPending'))
    expect(view.get('h1').text()).not.toBe(t('auth.authenticationFailed'))
    expect(view.text()).toContain(t('errors.api.ACCOUNT_RECOVERY_PENDING'))
    await view.get('button').trigger('click')
    expect(view.emitted('retry')).toHaveLength(1)
    await view.setProps({ code: 'INVALID_TELEGRAM_DATA', message: t('errors.api.INVALID_TELEGRAM_DATA') })
    expect(view.get('h1').text()).toBe(t('auth.authenticationFailed'))
    view.unmount()
  })
})
