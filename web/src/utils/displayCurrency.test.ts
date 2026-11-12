import { afterEach, describe, expect, it } from 'vitest'

import { convertTXBMoney, displayInputFromTXBMinor, displayInputMinimumFromTXBMinor, displayInputToTXBMinor, formatCatalogMoney, formatCatalogMoneyLines, formatMemberMoney, resetDisplayCurrency, setDisplayCurrency } from './displayCurrency'

const money = { currency: 'TXB', minor: '100', display: '1.00 TXB' } as const

describe('member display currency', () => {
  afterEach(() => resetDisplayCurrency())

  it('converts using fixed-point truncation instead of floating point rounding', () => {
    setDisplayCurrency({ currency: 'CNY', rates: { cnyTxbPerUnit: '3', usdTxbPerUnit: null } })

    expect(formatMemberMoney(money)).toBe('￥0.33')
    expect(formatCatalogMoney(money)).toBe('1.00 TXB (￥0.33)')
    expect(displayInputFromTXBMinor('300')).toBe('1.00')
    expect(displayInputToTXBMinor('1.00')).toBe('300')
    expect(displayInputMinimumFromTXBMinor('100')).toBe('0.34')
  })

  it('preserves signed values and uses USD when configured', () => {
    setDisplayCurrency({ currency: 'USD', rates: { cnyTxbPerUnit: null, usdTxbPerUnit: '2.5' } })

    expect(convertTXBMoney('-1250', 'USD', '2.5')).toBe('-$5.00')
    expect(formatMemberMoney({ currency: 'TXB', minor: '1250', display: '12.50 TXB' })).toBe('$5.00')
    expect(formatCatalogMoneyLines({ currency: 'TXB', minor: '1250', display: '12.50 TXB' })).toEqual({ primary: '12.50 TXB', approximation: '≈$5.00' })
  })

  it('retains the authoritative TXB value when a stored rate is unavailable', () => {
    setDisplayCurrency({ currency: 'CNY', rates: { cnyTxbPerUnit: null, usdTxbPerUnit: null } })

    expect(formatMemberMoney(money)).toBe('1.00 TXB')
    expect(displayInputToTXBMinor('1.01')).toBe('101')
  })

  it('preserves exact decimal input and rejects excess precision before conversion', () => {
    setDisplayCurrency({ currency: 'CNY', rates: { cnyTxbPerUnit: '3', usdTxbPerUnit: null } })
    expect(displayInputToTXBMinor('90071992547409.93')).toBe('27021597764222979')
    expect(displayInputToTXBMinor('1.005')).toBe('')
    expect(displayInputToTXBMinor('1e3')).toBe('')
    expect(displayInputToTXBMinor('')).toBe('')
  })

  it('falls back from zero USD to CNY, then to the authoritative TXB amount', () => {
    const smallMoney = { currency: 'TXB', minor: '1', display: '0.01 TXB' } as const

    setDisplayCurrency({ currency: 'USD', rates: { cnyTxbPerUnit: '0.2', usdTxbPerUnit: '1000' } })
    expect(formatMemberMoney(smallMoney)).toBe('￥0.05')

    setDisplayCurrency({ currency: 'CNY', rates: { cnyTxbPerUnit: '1000', usdTxbPerUnit: null } })
    expect(formatMemberMoney(smallMoney)).toBe('0.01 TXB')

    setDisplayCurrency({ currency: 'USD', rates: { cnyTxbPerUnit: null, usdTxbPerUnit: '1000' } })
    expect(formatMemberMoney(smallMoney)).toBe('0.01 TXB')
  })
})
