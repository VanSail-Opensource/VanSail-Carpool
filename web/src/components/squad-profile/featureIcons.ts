// Keep literal icon names discoverable by the icon compiler.
export const squadFeatureIcons = {
  route: 'i-ph-path',
  globe: 'i-ph-globe',
  lightning: 'i-ph-lightning',
  shield: 'i-ph-shield',
  sparkle: 'i-ph-sparkle',
  house: 'i-ph-house',
  broadcast: 'i-ph-broadcast',
  clock: 'i-ph-clock',
  stack: 'i-ph-stack',
  gauge: 'i-ph-gauge',
  server: 'i-ph-hard-drives',
  'arrows-clockwise': 'i-ph-arrows-clockwise',
  'rocket-launch': 'i-ph-rocket-launch',
  'cloud': 'i-ph-cloud',
  'wifi-high': 'i-ph-wifi-high',
  'planet': 'i-ph-planet',
  'compass': 'i-ph-compass',
  'map-pin': 'i-ph-map-pin',
  'devices': 'i-ph-devices',
  'desktop': 'i-ph-desktop',
  'timer': 'i-ph-timer',
  'lock-key': 'i-ph-lock-key',
  'check-circle': 'i-ph-check-circle',
  'lifebuoy': 'i-ph-lifebuoy',
  'heart': 'i-ph-heart',
  'leaf': 'i-ph-leaf',
  'lightbulb': 'i-ph-lightbulb',
  'cube': 'i-ph-cube',
  'link': 'i-ph-link',
  'code': 'i-ph-code',
  'funnel': 'i-ph-funnel',
  'faders': 'i-ph-faders',
  'sliders': 'i-ph-sliders',
  'arrows-left-right': 'i-ph-arrows-left-right',
  'database': 'i-ph-database',
  'globe-hemisphere-east': 'i-ph-globe-hemisphere-east',
} as const

export function squadFeatureIcon(icon: string): string {
  return Object.hasOwn(squadFeatureIcons, icon)
    ? squadFeatureIcons[icon as keyof typeof squadFeatureIcons]
    : 'i-ph-sparkle'
}
