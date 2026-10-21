import { z } from 'zod'
import type { DIYField } from '@/api/xnodeMember'
import { diyHeaders, diySockets } from './diyStructuredValidation'

export function diySchema(fields: DIYField[]) {
  const shape: Record<string, z.ZodType> = {}
  for (const field of fields) {
    let schema: z.ZodType
    const number = z.number().int().min(field.minimum).max(field.maximum)
    if (field.kind === 'boolean') schema = z.boolean()
    else if (field.kind === 'integer') schema = number
    else if (field.kind === 'enum-number') schema = number.refine(value => field.enum?.includes(String(value)) ?? false)
    else if (field.kind === 'enum') schema = z.string().refine(value => field.enum?.includes(value) ?? false)
    else if (field.kind === 'fast-open') schema = z.union([z.boolean(), number])
    else if (field.kind === 'range') schema = z.object({ min: z.number().int().min(0).max(2147483647), max: z.number().int().min(0).max(2147483647) }).strict().refine(value => value.min <= value.max && (!field.key.endsWith('XPaddingBytes') || value.min > 0 || value.max === 0))
    else if (field.kind === 'strings') schema = z.array(z.string().max(128)).max(64).refine(values => new Set(values).size === values.length && (!field.enum?.length || values.every(value => field.enum!.includes(value))))
    else if (field.kind === 'headers') schema = diyHeaders
    else if (field.kind === 'sockets') schema = diySockets
    else schema = z.string().max(field.maximum).refine(value => !/[\0\r\n]/.test(value))
    shape[field.key] = schema.optional()
  }
  return z.object(shape).strict().superRefine((value, ctx) => {
    for (const prefix of ['', 'download']) {
      const first = prefix ? 'downloadXmuxMaxConcurrency' : 'xmuxMaxConcurrency'
      const second = prefix ? 'downloadXmuxMaxConnections' : 'xmuxMaxConnections'
      const a = value[first] as { max: number } | undefined, b = value[second] as { max: number } | undefined
      if (a?.max && b?.max) ctx.addIssue({ code: 'custom', path: [second], message: 'conflict' })
    }
    if (value.xhttpMode === 'stream-one') for (const key of Object.keys(value)) if (key.startsWith('download')) ctx.addIssue({ code: 'custom', path: [key], message: 'conflict' })
    for (const [low, high] of [['minVersion', 'maxVersion'], ['downloadMinVersion', 'downloadMaxVersion']] as const) {
      const a = value[low], b = value[high]
      if (typeof a === 'string' && typeof b === 'string' && a > b) ctx.addIssue({ code: 'custom', path: [high], message: 'range' })
    }
    if (value.penetrate === true) for (const field of fields) if (field.group === 'download' && /^(download(DomainStrategy|Tcp|Interface|Mark|Happy|Custom))/.test(field.key) && value[field.key] !== undefined) ctx.addIssue({ code: 'custom', path: [field.key], message: 'conflict' })
    for (const prefix of ['singboxMux', 'mihomoMux']) if (Number(value[prefix + 'MaxStreams']) > 0 && (Number(value[prefix + 'MaxConnections']) > 0 || Number(value[prefix + 'MinStreams']) > 0)) ctx.addIssue({ code: 'custom', path: [prefix + 'MaxStreams'], message: 'conflict' })
    if (value.ssUDPOverTCPVersion !== undefined && value.ssUDPOverTCP !== true) ctx.addIssue({ code: 'custom', path: ['ssUDPOverTCPVersion'], message: 'dependency' })
    for (const key of ['ssUDPOverTCP', 'singboxMuxEnabled', 'mihomoMuxEnabled']) if (value[key] === true) ctx.addIssue({ code: 'custom', path: [key], message: 'serverCapability' })
  })
}
