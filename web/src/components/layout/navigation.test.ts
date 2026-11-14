import { describe, expect, it } from 'vitest'

import { desktopNavigationItems, mobileNavigationItems } from './navigation'

describe('member navigation access gating', () => {
  it('keeps primary navigation small while making education available to new users', () => {
    expect(mobileNavigationItems(false, false, true).map(item => item.to)).toEqual(['/home', '/catalog'])
    const desktop = desktopNavigationItems(key => key, false, false, false, false, false, true)
    expect(desktop.filter(item => item.to).map(item => item.to)).toEqual(['/home', '/catalog'])
    expect(desktop.find(item => item.value === 'around-tx')?.children?.map(item => item.to)).toEqual(['/knowledgebase'])
  })

  it('keeps education available when optional paid tools are hidden', () => {
    const around = desktopNavigationItems(key => key, false).find(item => item.value === 'around-tx')
    expect(around?.children?.map(item => item.to)).toEqual(['/knowledgebase'])
  })

  it('keeps Settings for members with access', () => {
    expect(mobileNavigationItems(false, false, false).map(item => item.to)).toContain('/settings')
    expect(desktopNavigationItems(key => key, false, false, false, false, true, false).some(item => item.to === '/settings')).toBe(true)
  })
})
