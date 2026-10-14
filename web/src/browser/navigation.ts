import type { NavigationMenuItem } from '@nuxt/ui'

export const adminGroups = [
  { labelKey: 'adminNav.accounts', sections: [
    { value: 'users', labelKey: 'adminNav.users', icon: 'i-ph-users-bold' },
    { value: 'affiliates', labelKey: 'adminNav.affiliates', icon: 'i-ph-handshake-bold' },
    { value: 'questionnaires', labelKey: 'adminNav.questionnaires', icon: 'i-ph-list-checks-bold' },
    { value: 'onboarding', labelKey: 'adminNav.onboarding', icon: 'i-ph-user-plus-bold' },
  ] },
  { labelKey: 'adminNav.commerce', sections: [
    { value: 'catalog', labelKey: 'adminNav.catalog', icon: 'i-ph-package-bold' },
    { value: 'coupons', labelKey: 'adminNav.coupons', icon: 'i-ph-ticket-bold' },
    { value: 'usdt', labelKey: 'adminUsdt.title', icon: 'i-ph-currency-circle-dollar-bold' },
    { value: 'activity', labelKey: 'adminNav.activity', icon: 'i-ph-game-controller-bold' },
  ] },
  { labelKey: 'browser.services', sections: [
    { value: 'xnode', labelKey: 'xnode.title', icon: 'i-ph-hard-drives-bold' },
    { value: 'host-connectivity', labelKey: 'hostConnectivity.title', icon: 'i-ph-plugs-connected-bold' },
    { value: 'ip-lookup', labelKey: 'ipLookup.adminTitle', icon: 'i-ph-globe-bold' },
    { value: 'compensation', labelKey: 'adminCompensation.nav', icon: 'i-ph-first-aid-bold' },
    { value: 'abuse', labelKey: 'adminAbuse.nav', icon: 'i-ph-shield-warning-bold' },
  ] },
  { labelKey: 'adminNav.system', sections: [
    { value: 'settings', labelKey: 'adminNav.settings', icon: 'i-ph-gear-six-bold' },
    { value: 'backups', labelKey: 'adminNav.backups', icon: 'i-ph-archive-bold' },
    { value: 'database', labelKey: 'adminNav.database', icon: 'i-ph-database-bold' },
    { value: 'audit', labelKey: 'adminNav.audit', icon: 'i-ph-clock-counter-clockwise-bold' },
  ] },
] as const

export function consoleNavigation(t: (key: string) => string): NavigationMenuItem[][] {
  return adminGroups.map(group => [
    { label: t(group.labelKey), type: 'label' },
    ...group.sections.map(section => ({ label: t(section.labelKey), 'aria-label': t(section.labelKey), to: `/admin/${section.value}`, icon: section.icon })),
  ])
}
