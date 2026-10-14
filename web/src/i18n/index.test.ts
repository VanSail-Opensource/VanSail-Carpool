import { afterEach, describe, expect, it } from 'vitest'

import { getLocale, setLocale, t } from './index'

describe('i18n', () => {
  afterEach(() => {
    localStorage.removeItem('tx-carpool-locale')
    setLocale('en')
  })

  it('interpolates locale placeholders and persists switching', () => {
    setLocale('zh-CN')
    expect(getLocale()).toBe('zh-CN')
    expect(localStorage.getItem('tx-carpool-locale')).toBe('zh-CN')
    expect(t('activity.groupRewardProgress', { count: 2, threshold: 10 })).toContain('2')
    expect(t('activity.groupRewardProgress', { count: 2, threshold: 10 })).toContain('10')
  })

  it('falls back to English for an unavailable key value', () => {
    setLocale('zh-CN')
    expect(t('common.refresh')).not.toBe('common.refresh')
  })

  it('includes localized recovery-boundary copy', () => {
    expect(t('recovery.title')).toBe('VanSail Carpool could not open')
    setLocale('zh-CN')
    expect(t('recovery.title')).toBe('VanSail Carpool 无法打开')
  })
})
