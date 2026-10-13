import { describe, expect, it } from 'vitest'
import { getLocale, setLocale, t } from './index'

describe('crypto payment localization', () => {
  it('resolves dotted channel keys and preserves ordinary nested lookup', () => {
    const before = getLocale()
    for (const locale of ['en', 'zh-CN'] as const) {
      setLocale(locale)
      expect(t('payment.channels.usdt.erc20')).not.toBe('payment.channels.usdt.erc20')
      expect(t('payment.channels.usdt.trc20')).not.toBe('payment.channels.usdt.trc20')
      expect(t('adminUsdt.states.ready')).not.toBe('adminUsdt.states.ready')
      expect(t('missing.translation')).toBe('missing.translation')
    }
    setLocale(before)
  })
})
