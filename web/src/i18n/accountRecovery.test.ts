import { afterEach, describe, expect, it } from 'vitest'
import { localizedError, setLocale } from './index'

afterEach(() => setLocale('en'))

describe('verified account cleanup recovery', () => {
  it.each(['en', 'zh-CN'] as const)('explains paused cleanup without rejecting identity in %s', locale => {
    setLocale(locale)
    const recovery = localizedError({ code: 'ACCOUNT_RECOVERY_PENDING' }, 'auth.authenticationFailed')
    const invalid = localizedError({ code: 'INVALID_TELEGRAM_DATA' }, 'auth.authenticationFailed')
    expect(recovery).not.toBe(invalid)
    expect(recovery).not.toContain('ACCOUNT_RECOVERY_PENDING')
    expect(recovery).toContain(locale === 'en' ? 'sign-in was verified' : '身份已验证')
    expect(recovery).toContain(locale === 'en' ? 'administrator' : '管理员')
  })
})
