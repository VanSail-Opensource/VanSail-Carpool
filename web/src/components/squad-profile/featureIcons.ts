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
} as const

export function squadFeatureIcon(icon: string): string {
  return Object.hasOwn(squadFeatureIcons, icon)
    ? squadFeatureIcons[icon as keyof typeof squadFeatureIcons]
    : 'i-ph-sparkle'
}
