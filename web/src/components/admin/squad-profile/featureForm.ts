import type { SquadProductWrite } from '@/api/types'
import { squadFeatureIcons } from '@/components/squad-profile/featureIcons'

export type SquadFeature = NonNullable<SquadProductWrite['features']>[number]
export const maxSquadFeatures = 12

export function validSquadFeatures(features: readonly SquadFeature[]): boolean {
  return features.length <= maxSquadFeatures && features.every((feature) => {
    const titleLength = Array.from(feature.title.trim()).length
    const detailLength = Array.from((feature.detail ?? '').trim()).length
    return Object.hasOwn(squadFeatureIcons, feature.icon) && titleLength > 0 && titleLength <= 80 && detailLength <= 160
  })
}

export function normalizedSquadFeatures(features: readonly SquadFeature[]): SquadFeature[] {
  return features.map((feature) => ({
    icon: feature.icon,
    title: feature.title.trim(),
    ...(feature.detail?.trim() ? { detail: feature.detail.trim() } : {}),
  }))
}

// Omit untouched features so a stale editor cannot overwrite newer changes.
export function squadFeaturesPatch(features: readonly SquadFeature[], touched: boolean): Pick<SquadProductWrite, 'features'> {
  return touched ? { features: normalizedSquadFeatures(features) } : {}
}
