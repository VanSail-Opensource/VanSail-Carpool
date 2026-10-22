import type { DIYField, DIYOverrides } from '@/api/xnodeMember'

export function diyFieldActive(field: DIYField, draft: DIYOverrides) {
  const values = draft as Record<string, unknown>
  return field.available !== false && (field.dependencies ?? []).every(rule => {
    const candidates = values[rule.key] === undefined && rule.inheritedValues?.length ? rule.inheritedValues : [String(values[rule.key])]
    return candidates.some(value => (!rule.values?.length || rule.values.includes(value)) && !rule.notValues?.includes(value))
  })
}

export function activeDIYOverrides(fields: DIYField[], draft: DIYOverrides): DIYOverrides {
  return Object.fromEntries(fields.filter(field => diyFieldActive(field, draft) && field.key in draft).map(field => [field.key, (draft as Record<string, unknown>)[field.key]])) as DIYOverrides
}
