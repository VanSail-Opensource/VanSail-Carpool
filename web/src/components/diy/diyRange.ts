export type DIYRange = { min: number; max: number }

export function parseDIYRange(text: string): DIYRange | undefined {
  const match = /^(\d+)(?:\s*-\s*(\d+))?$/.exec(text.trim())
  if (!match) return undefined
  const min = Number(match[1]), max = Number(match[2] ?? match[1])
  return Number.isSafeInteger(min) && Number.isSafeInteger(max) && min >= 0 && min <= max && max <= 2147483647 ? { min, max } : undefined
}

export function formatDIYRange(value: unknown): string {
  if (typeof value === 'string') return value
  if (!value || typeof value !== 'object' || !('min' in value) || !('max' in value)) return ''
  return value.min === value.max ? String(value.min) : `${value.min}-${value.max}`
}
