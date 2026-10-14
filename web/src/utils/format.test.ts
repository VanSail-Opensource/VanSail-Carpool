import { describe, expect, it } from 'vitest'

import { formatBytes, formatMoney, moneyFromTxbInput, signedMoneyFromTxbInput, trafficBytesFromInput, txbInputFromMinor } from './format'

describe('format utilities', () => {
  it('uses the server display amount without recalculating prices', () => {
    expect(formatMoney({ currency: 'TXB', minor: '1250', display: '12.50 TXB' })).toBe('12.50 TXB')
  })

  it('uses currency symbols for CNY and USD amounts', () => {
    expect(formatMoney({ currency: 'CNY', minor: '125', display: '1.25 CNY' })).toBe('￥1.25')
    expect(formatMoney({ currency: 'USD', minor: '-125', display: '-1.25 USD' })).toBe('-$1.25')
  })

  it('parses user TXB input into integer hundredths', () => {
    expect(moneyFromTxbInput('20')).toBe('2000')
    expect(moneyFromTxbInput('20.5')).toBe('2050')
    expect(moneyFromTxbInput('20.005')).toBe('')
    expect(moneyFromTxbInput('-2')).toBe('')
    expect(moneyFromTxbInput('150')).toBe('15000')
    expect(txbInputFromMinor('15000')).toBe('150.00')
  })

  it('parses signed reward amounts into integer hundredths', () => {
    expect(signedMoneyFromTxbInput('-1.25')).toBe('-125')
    expect(signedMoneyFromTxbInput('+2')).toBe('200')
    expect(signedMoneyFromTxbInput('1.234')).toBe('')
  })

  it('formats byte counts without mutating the source value', () => {
    expect(formatBytes('1073741824')).toBe('1 GiB')
    expect(formatBytes('1610612736')).toBe('1.5 GiB')
    expect(formatBytes('1342177280')).toBe('1.25 GiB')
    expect(formatBytes(0)).toBe('0 B')
  })

  it('converts binary traffic units to exact bytes', () => {
    expect(trafficBytesFromInput('10GB')).toBe('10737418240')
    expect(trafficBytesFromInput('10GiB')).toBe('10737418240')
    expect(trafficBytesFromInput('9007199254740993 B')).toBe('9007199254740993')
    expect(trafficBytesFromInput('100 MB')).toBe('104857600')
    expect(trafficBytesFromInput('1.5 KB')).toBe('1536')
    expect(trafficBytesFromInput('1.1 B')).toBe('')
  })
})
