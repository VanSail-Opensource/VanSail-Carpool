import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { FeaturePaymentReturnStatus } from '@/api/features'
import PaymentReceiptDetails from './PaymentReceiptDetails.vue'
import WalletPaymentSuccess from './WalletPaymentSuccess.vue'

vi.mock('@/composables/useMotionPreferences', async () => {
  const { shallowRef } = await import('vue')
  return { useMotionPreferences: () => ({ reducedMotion: shallowRef(false) }) }
})

const receipt: FeaturePaymentReturnStatus = {
  id: 'payment-1', provider: 'bepusdt', providerRail: 'usdt.trc20', status: 'pending',
  txb: { currency: 'TXB', minor: '2000', display: '20.00 TXB' }, payableAmount: '20.00', payableCurrency: 'CNY',
  actualCryptoAmount: '2.000001', actualCryptoCurrency: 'USDT', createdAt: '2026-11-01T00:00:00Z', paidAt: null,
}

describe('payment receipt amount evidence', () => {
  it.each(['creating', 'pending', 'cancelled', 'expired', 'failed', 'refunded'] as const)('does not claim credited balance for a %s receipt', status => {
    const wrapper = mount(PaymentReceiptDetails, { props: { title: 'Receipt', description: '', state: status === 'pending' ? 'pending' : 'terminal', details: { ...receipt, status }, canReissue: false } })
    expect(wrapper.get('.payment-receipt__label').text()).toBe('Order value in TXB')
    expect(wrapper.text()).toContain('20.00 TXB')
    expect(wrapper.text()).toContain('Crypto amount')
    expect(wrapper.text()).not.toContain('Balance credited')
    expect(wrapper.text()).not.toContain('Settled amount')
    wrapper.unmount()
  })

  it('renders a receipt first seen paid as static credited balance', () => {
    const wrapper = mount(PaymentReceiptDetails, { props: { title: 'Receipt', description: '', state: 'confirmed', details: { ...receipt, status: 'paid', paidAt: '2026-11-01T00:01:00Z' }, canReissue: false } })
    expect(wrapper.get('.payment-receipt__label').text()).toBe('Balance credited')
    expect(wrapper.findComponent(WalletPaymentSuccess).exists()).toBe(false)
    wrapper.unmount()
  })

  it('animates confirmed fiat returns without hiding metadata or retaining denied details', async () => {
    const focus = vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    const fiat = { ...receipt, provider: 'ezpay', providerRail: 'alipay', actualCryptoAmount: null, actualCryptoCurrency: null } satisfies FeaturePaymentReturnStatus
    const wrapper = mount(PaymentReceiptDetails, { props: { title: 'Receipt', description: '', state: 'pending', details: fiat, canReissue: false } })
    try {
      expect(wrapper.findComponent(WalletPaymentSuccess).exists()).toBe(false)
      await wrapper.setProps({ details: { ...fiat, status: 'paid', paidAt: '2026-11-01T00:01:00Z' } })
      expect(wrapper.findComponent(WalletPaymentSuccess).exists()).toBe(false)
      await wrapper.setProps({ state: 'confirmed' })
      const animation = wrapper.getComponent(WalletPaymentSuccess)
      expect(animation.props()).toMatchObject({ amount: fiat.txb, phase: 'waiting', hideDone: true })
      expect(wrapper.text()).toContain(fiat.id)
      expect(wrapper.text()).toContain('20.00 CNY')
      await wrapper.setProps({ state: 'unavailable' })
      expect(wrapper.findComponent(WalletPaymentSuccess).exists()).toBe(false)
      expect(wrapper.text()).not.toContain(fiat.id)
      expect(wrapper.text()).not.toContain('20.00 TXB')
      expect(wrapper.text()).not.toContain('20.00 CNY')
    } finally {
      wrapper.unmount()
      focus.mockRestore()
    }
  })
})
