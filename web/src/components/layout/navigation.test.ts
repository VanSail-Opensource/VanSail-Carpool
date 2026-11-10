import { describe, expect, it } from 'vitest'

import { desktopNavigationItems, mobileNavigationItems } from './navigation'

describe('member navigation access gating', () => {
  it('keeps only Home and Explore for new users', () => {
    expect(mobileNavigationItems(false, false, true).map(item => item.to)).toEqual(['/home', '/catalog'])
    expect(desktopNavigationItems(key => key, false, false, false, false, false, true).map(item => item.to)).toEqual(['/home', '/catalog'])
  })

  it('keeps Settings for members with access', () => {
    expect(mobileNavigationItems(false, false, false).map(item => item.to)).toContain('/settings')
    expect(desktopNavigationItems(key => key, false, false, false, false, true, false).some(item => item.to === '/settings')).toBe(true)
  })
})
