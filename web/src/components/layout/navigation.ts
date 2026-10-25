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



export function mobileNavigationItems(_isAdmin: boolean, showActivity = false): MobileNavigationItem[] {
  const visible = mobileNavigation.filter(item => item.to !== '/activity' || showActivity)
  return [...visible]
}

export function desktopNavigationItems(t: Translate, _isAdmin: boolean, showAroundTX = false, showActivity = false, showIPLookup = false): NavigationMenuItem[] {
  const items: NavigationMenuItem[] = [
    {
      label: t('nav.home'), to: '/home', icon: 'i-ph-house', defaultOpen: true, exactQuery: true,
      children: [
        { label: t('subscription.title'), to: '/subscription', icon: 'i-ph-link-bold' },
        { label: t('nav.connections'), to: '/connections', icon: 'i-ph-devices' },
        { label: t('nav.revoke'), to: { path: '/subscription', query: { revoke: '1' } }, icon: 'i-ph-trash', exactQuery: true },
      ],
    },
    { label: t('nav.explore'), to: '/catalog', icon: 'i-ph-compass' },
    ...(showActivity ? [{ label: t('nav.activity'), to: '/activity', icon: 'i-ph-game-controller' }] : []),
    { label: t('settings.title'), to: '/settings', icon: 'i-ph-gear' },
  ]
  if (showAroundTX || showIPLookup) {
    items.splice(1, 0, {
      label: t('nav.aroundTx'), icon: 'i-ph-sparkle', type: 'trigger', defaultOpen: true, popover: true,
      children: [
        ...(showIPLookup ? [{ label: t('ipLookup.title'), to: '/ip-lookup', icon: 'i-ph-globe-hemisphere-west' }] : []),
        ...(showAroundTX ? [
        { label: t('affiliates.title'), to: '/affiliates', icon: 'i-ph-users-three' },
        { label: t('nav.questionnaire'), to: '/questionnaire', icon: 'i-ph-list-checks' },
        { label: t('community.title'), to: '/community', icon: 'i-ph-users-three' },
        { label: t('nav.emby'), to: '/emby', icon: 'i-ph-monitor-play' },
        ] : []),
      ],
    })
  }
  return items
}
