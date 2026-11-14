import type { NavigationMenuItem } from '@nuxt/ui'

type Translate = (key: string) => string

interface MobileNavigationItem {
  labelKey: string
  to: string
  icon: string
}



const mobileNavigation: readonly MobileNavigationItem[] = [
  { labelKey: 'nav.home', to: '/home', icon: 'i-ph-house' },
  { labelKey: 'nav.explore', to: '/catalog', icon: 'i-ph-compass' },
  { labelKey: 'nav.activity', to: '/activity', icon: 'i-ph-game-controller' },
  { labelKey: 'settings.title', to: '/settings', icon: 'i-ph-gear' },
]



// A new user has no server-confirmed access yet, so the only useful member
// routes are Home and Explore. The allow-list is authoritative so the result
// cannot leak Settings, Activity or Around TX through stale preference flags.
// Administrators never count as new users because their one-use console
// entrance lives in Settings.
export function mobileNavigationItems(_isAdmin: boolean, showActivity = false, isNewUser = false): MobileNavigationItem[] {
  if (isNewUser) return mobileNavigation.filter(item => item.to === '/home' || item.to === '/catalog')
  const visible = mobileNavigation.filter(item => item.to !== '/activity' || showActivity)
  return [...visible]
}

export function desktopNavigationItems(t: Translate, _isAdmin: boolean, showAroundTX = false, showActivity = false, showIPLookup = false, showSubscription = true, isNewUser = false): NavigationMenuItem[] {
  if (isNewUser) {
    return [
      { value: 'home', label: t('nav.home'), to: '/home', icon: 'i-ph-house', exactQuery: true },
      { value: 'explore', label: t('nav.explore'), to: '/catalog', icon: 'i-ph-compass' },
    ]
  }
  const subscriptionChildren = showSubscription
    ? [
        { value: 'subscription', label: t('subscription.title'), to: '/subscription', icon: 'i-ph-link-bold' },
        { value: 'connections', label: t('nav.connections'), to: '/connections', icon: 'i-ph-devices' },
        { value: 'revoke', label: t('nav.revoke'), to: { path: '/subscription', query: { revoke: '1' } }, icon: 'i-ph-trash', exactQuery: true },
      ]
    : []
  const items: NavigationMenuItem[] = [
    {
      value: 'home', label: t('nav.home'), to: '/home', icon: 'i-ph-house', defaultOpen: showSubscription, exactQuery: true,
      ...(subscriptionChildren.length ? { children: subscriptionChildren } : {}),
    },
    { value: 'explore', label: t('nav.explore'), to: '/catalog', icon: 'i-ph-compass' },
    ...(showActivity ? [{ value: 'activity', label: t('nav.activity'), to: '/activity', icon: 'i-ph-game-controller' }] : []),
    { value: 'settings', label: t('settings.title'), to: '/settings', icon: 'i-ph-gear' },
  ]
  if (showAroundTX || showIPLookup) {
    items.splice(1, 0, {
      value: 'around-tx', label: t('nav.aroundTx'), icon: 'i-ph-sparkle', type: 'trigger', defaultOpen: true, popover: true,
      children: [
        ...(showIPLookup ? [{ value: 'ip-lookup', label: t('ipLookup.title'), to: '/ip-lookup', icon: 'i-ph-globe-hemisphere-west' }] : []),
        ...(showAroundTX ? [
        { value: 'affiliates', label: t('affiliates.title'), to: '/affiliates', icon: 'i-ph-users-three' },
        { value: 'community', label: t('community.title'), to: '/community', icon: 'i-ph-users-three' },
        { value: 'emby', label: t('nav.emby'), to: '/emby', icon: 'i-ph-monitor-play' },
        ] : []),
      ],
    })
  }
  return items
}
