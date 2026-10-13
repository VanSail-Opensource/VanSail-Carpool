import type { NavigationMenuItem } from '@nuxt/ui'

type Translate = (key: string) => string

interface MobileNavigationItem {
  labelKey: string
  to: string
  icon: string
}

interface AdminNavigationItem {
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

const adminNavigation: readonly AdminNavigationItem[] = [
  { labelKey: 'adminUsdt.title', to: '/admin/usdt', icon: 'i-ph-currency-circle-dollar' },
  { labelKey: 'ipLookup.adminTitle', to: '/admin/ip-lookup', icon: 'i-ph-globe-hemisphere-west' },
  { labelKey: 'adminNav.catalog', to: '/admin/catalog', icon: 'i-ph-package' },
  { labelKey: 'adminNav.coupons', to: '/admin/coupons', icon: 'i-ph-ticket' },
  { labelKey: 'adminNav.activity', to: '/admin/activity', icon: 'i-ph-game-controller' },
  { labelKey: 'adminNav.affiliates', to: '/admin/affiliates', icon: 'i-ph-users-three' },
  { labelKey: 'adminNav.questionnaires', to: '/admin/questionnaires', icon: 'i-ph-file-csv' },
  { labelKey: 'adminNav.onboarding', to: '/admin/onboarding', icon: 'i-ph-user-plus-bold' },
  { labelKey: 'adminNav.users', to: '/admin/users', icon: 'i-ph-user-focus' },
  { labelKey: 'adminNav.settings', to: '/admin/settings', icon: 'i-ph-key' },
  { labelKey: 'hostConnectivity.title', to: '/admin/host-connectivity', icon: 'i-ph-plugs-connected' },
  { labelKey: 'adminCompensation.nav', to: '/admin/compensation', icon: 'i-ph-first-aid' },
  { labelKey: 'adminAbuse.nav', to: '/admin/abuse', icon: 'i-ph-shield-warning' },
  { labelKey: 'adminNav.backups', to: '/admin/backups', icon: 'i-ph-archive' },
  { labelKey: 'adminNav.database', to: '/admin/database', icon: 'i-ph-database' },
  { labelKey: 'adminNav.audit', to: '/admin/audit', icon: 'i-ph-shield-check' },
]

export function mobileNavigationItems(isAdmin: boolean, showActivity = false): MobileNavigationItem[] {
  const visible = mobileNavigation.filter(item => item.to !== '/activity' || showActivity)
  return isAdmin
    ? [...visible, { labelKey: 'nav.admin', to: '/admin/settings', icon: 'i-ph-shield-check' }]
    : [...visible]
}

export function desktopNavigationItems(t: Translate, isAdmin: boolean, showAroundTX = false, showActivity = false, showIPLookup = false): NavigationMenuItem[] {
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
        { label: t('statistics.title'), to: '/statistics', icon: 'i-ph-chart-donut' },
        { label: t('abuse.title'), to: '/abuse-records', icon: 'i-ph-shield-warning' },
        ] : []),
      ],
    })
  }
  if (isAdmin) {
    items.push({
      label: t('nav.admin'), icon: 'i-ph-shield-check', type: 'trigger', defaultOpen: true, popover: true,
      children: adminNavigation.map((item) => ({ label: t(item.labelKey), to: item.to, icon: item.icon })),
    })
  }
  return items
}
