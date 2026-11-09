import { z } from 'zod'
import type { XNodeEndpoint, XNodeEndpointDraft, XNodeEndpointField } from '@/api/xnode'
import { clientHeaders } from '@/components/diy/diyStructuredValidation'
import { objectOption } from './nodeFormOptions'

export type EndpointDraft = Omit<XNodeEndpointDraft, 'overrides'> & { overrides: Record<string, unknown> }
export function endpointDraft(item?: XNodeEndpoint, nodeId = ''): EndpointDraft {
  return { id: item?.id ?? '', nodeId: item?.nodeId ?? nodeId, inboundTag: item?.inboundTag ?? '', name: item?.name ?? '', address: item?.address ?? '', port: item?.port ?? 443, enabled: item?.enabled ?? true, hidden: item?.hidden ?? false, overrides: JSON.parse(JSON.stringify(item?.overrides ?? {})) }
}
export function endpointPayload(draft: EndpointDraft, fields?: XNodeEndpointField[]): XNodeEndpointDraft {
  const body = JSON.parse(JSON.stringify(draft)) as XNodeEndpointDraft
  // Keep incomplete numeric input local; the draft endpoint validates integer 0.
  if (!Number.isSafeInteger(body.port)) body.port = 0
  if (fields && body.overrides) {
    const values = objectOption(body.overrides.clientOptions)
    body.overrides.clientOptions = Object.fromEntries(fields.filter(field => endpointOptionActive(field, values) && field.key in values).map(field => [field.key, values[field.key]]))
  }
  return body
}
export function endpointOptions(draft: EndpointDraft): Record<string, unknown> { return objectOption(draft.overrides.clientOptions) }
export function endpointOptionActive(field: XNodeEndpointField, values: Record<string, unknown>): boolean {
  if (field.key === 'hysteriaHopInterval' && Array.isArray(values.hysteriaPorts) && !values.hysteriaPorts.length) return false
  return field.available !== false && (field.dependencies ?? []).every(rule => {
    const candidates = values[rule.key] === undefined && rule.inheritedValues?.length ? rule.inheritedValues : [String(values[rule.key])]
    return candidates.some(value => (!rule.values?.length || rule.values.includes(value)) && !rule.notValues?.includes(value))
  })
}
export function endpointOptionErrors(fields: XNodeEndpointField[], values: Record<string, unknown>): Record<string, string> {
  const active = fields.filter(field => endpointOptionActive(field, values))
  const errors: Record<string, string> = {}
  for (const field of active.filter(field => values[field.key] !== undefined)) {
    const number = z.number().int().min(field.minimum).max(field.maximum)
    const range = z.object({ min: z.number().int().min(field.minimum).max(field.maximum), max: z.number().int().min(field.minimum).max(field.maximum) }).strict().refine(value => value.min <= value.max)
    let schema: z.ZodType = z.string().max(field.maximum).refine(value => !/[\0\r\n]/.test(value))
    if (field.kind === 'boolean') schema = field.restrictions?.includes('fixedOff') ? z.literal(false) : z.boolean()
    else if (field.kind === 'integer') schema = number
    else if (field.kind === 'enum-number') schema = number.refine(value => field.enum?.includes(String(value)) ?? false)
    else if (field.kind === 'enum') schema = z.string().refine(value => field.enum?.includes(value) ?? false)
    else if (field.kind === 'fast-open') schema = z.union([z.boolean(), number])
    else if (field.kind === 'range') schema = range
    else if (field.kind === 'port-ranges') schema = z.array(range).max(64)
    else if (field.kind === 'strings') schema = z.array(z.string().max(128)).max(64).refine(rows => new Set(rows).size === rows.length && (!field.enum?.length || rows.every(row => field.enum!.includes(row))))
    else if (field.kind === 'headers') schema = clientHeaders.refine(rows => rows.every(row => row.name.toLowerCase() !== 'host'))
    if (!schema.safeParse(values[field.key]).success) errors[field.key] = field.kind === 'port-ranges' ? 'ports' : 'invalid'
  }
  return errors
}
