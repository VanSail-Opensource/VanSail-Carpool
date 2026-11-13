import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { FeaturePaymentReturnStatus } from '@/api/features'
import PaymentReceiptDetails from './PaymentReceiptDetails.vue'

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

  it('uses credited language only after the server reports paid', () => {
    const wrapper = mount(PaymentReceiptDetails, { props: { title: 'Receipt', description: '', state: 'confirmed', details: { ...receipt, status: 'paid', paidAt: '2026-11-01T00:01:00Z' }, canReissue: false } })
    expect(wrapper.get('.payment-receipt__label').text()).toBe('Balance credited')
    wrapper.unmount()
  })
})
