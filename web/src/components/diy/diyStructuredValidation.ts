import { z } from 'zod'

const integer = (value: string, optional = false, signed = false) => optional && value === '' || (signed ? /^-?\d+$/ : /^\d+$/).test(value) && Number(value) >= (signed ? -2147483648 : 0) && Number(value) <= 2147483647
const protectedHeaders = new Set(['host', 'authorization', 'proxy-authorization', 'cookie', 'content-length', 'transfer-encoding', 'connection', 'upgrade', 'content-type'])
export const diyHeaders = z.array(z.object({ name: z.string().min(1).max(128).regex(/^[!#$%&'*+.^_`|~\da-z-]+$/i), value: z.string().max(4096).refine(value => [...value].every(char => char.charCodeAt(0) === 9 || char.charCodeAt(0) >= 32 && char.charCodeAt(0) !== 127)) }).strict()).max(32).refine(rows => {
  const keys = rows.map(row => row.name.toLowerCase())
  return keys.every(key => !protectedHeaders.has(key)) && new Set(keys).size === keys.length
})
export const diySockets = z.array(z.object({ system: z.enum(['', 'linux', 'android', 'windows', 'darwin', 'ios', 'freebsd']), network: z.enum(['', 'tcp', 'tcp4', 'tcp6', 'udp', 'udp4', 'udp6']), level: z.string().max(16).refine(value => integer(value, true)), opt: z.string().max(16).refine(value => integer(value)), value: z.string().max(512), type: z.enum(['int', 'str']) }).strict().refine(row => !(row.system === 'windows' && row.type === 'str') && (row.type !== 'int' || integer(row.value, false, true)))).max(32)
