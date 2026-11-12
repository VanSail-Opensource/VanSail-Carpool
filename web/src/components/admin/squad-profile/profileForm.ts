import type { SquadProfile, SquadProfileWrite } from '@/api/types'
import { isCountryCode, normalizeCountryCode, type ProfileType } from '@/components/squad-profile/profile'

export interface ProfileDraft {
  type: ProfileType
  isp: string
  portMbps: number | undefined
  dynamic: boolean
  location: string
  ct: string
  cu: string
  cm: string
  countryCode: string
  upstreamCarriers: string
  unlimited: boolean
}

export function updateDraft(draft: ProfileDraft, patch: Partial<ProfileDraft>): ProfileDraft {
  return { ...draft, ...patch }
}

const blankDraft = (): ProfileDraft => ({
  type: 'international_network', isp: '', portMbps: undefined, dynamic: false, location: '',
  ct: '', cu: '', cm: '', countryCode: '', upstreamCarriers: '', unlimited: true,
})

export function draftFromProfile(profile: SquadProfile | null): ProfileDraft {
  const draft = blankDraft()
  if (!profile) return draft
  draft.type = profile.type
  if (profile.type === 'broadband') Object.assign(draft, { isp: profile.isp, portMbps: profile.portMbps, dynamic: profile.dynamic, location: profile.location, unlimited: false })
  if (profile.type === 'china_optimized') Object.assign(draft, { ct: profile.ct, cu: profile.cu, cm: profile.cm, portMbps: profile.portMbps ?? undefined, countryCode: normalizeCountryCode(profile.countryCode), unlimited: profile.portMbps === null })
  if (profile.type === 'international_network') Object.assign(draft, { portMbps: profile.portMbps ?? undefined, countryCode: normalizeCountryCode(profile.countryCode), upstreamCarriers: profile.upstreamCarriers.join(', '), unlimited: profile.portMbps === null })
  return draft
}

function normalizedPort(draft: ProfileDraft): number | null | undefined {
  if (draft.type !== 'broadband' && draft.unlimited) return null
  if (!Number.isSafeInteger(draft.portMbps) || (draft.portMbps ?? 0) < 1) return undefined
  return draft.portMbps
}

function carriers(value: string): string[] {
  return [...new Set(value.split(',').map((item) => item.trim()).filter(Boolean))]
}

export function profileFromDraft(draft: ProfileDraft): SquadProfileWrite | null {
  if (draft.type === 'broadband') {
    if (!draft.isp.trim() || !draft.location.trim() || normalizedPort(draft) === undefined) return null
    return { type: 'broadband', isp: draft.isp.trim(), portMbps: draft.portMbps as number, dynamic: draft.dynamic, location: draft.location.trim() }
  }
  const port = normalizedPort(draft)
  const countryCode = normalizeCountryCode(draft.countryCode)
  if (!isCountryCode(countryCode) || port === undefined) return null
  if (draft.type === 'china_optimized') {
    if (!draft.ct.trim() || !draft.cu.trim() || !draft.cm.trim()) return null
    return { type: 'china_optimized', ct: draft.ct.trim(), cu: draft.cu.trim(), cm: draft.cm.trim(), portMbps: port, countryCode }
  }
  const upstreamCarriers = carriers(draft.upstreamCarriers)
  if (!upstreamCarriers.length) return null
  return { type: 'international_network', portMbps: port, countryCode, upstreamCarriers }
}

export function editableProfile(profile: SquadProfile | null): SquadProfileWrite | null {
  if (!profile) return null
  if (profile.type === 'broadband') return { ...profile }
  if (profile.type === 'china_optimized') return { ...profile }
  return { ...profile, upstreamCarriers: [...profile.upstreamCarriers] }
}

export function validationKey(draft: ProfileDraft): string {
  if (draft.type === 'broadband' && normalizedPort(draft) === undefined) return 'squadProfile.validation.port'
  if (draft.type !== 'broadband' && !isCountryCode(draft.countryCode)) return 'squadProfile.validation.country'
  if (draft.type === 'international_network' && !carriers(draft.upstreamCarriers).length) return 'squadProfile.validation.carriers'
  return 'squadProfile.validation.required'
}
