import { z } from 'zod'

const protectedHeaders = new Set(['host', 'authorization', 'proxy-authorization', 'cookie', 'content-length', 'transfer-encoding', 'connection', 'upgrade', 'content-type'])
export const diyHeaders = z.array(z.object({ name: z.string().min(1).max(128).regex(/^[!#$%&'*+.^_`|~\da-z-]+$/i), value: z.string().max(4096).refine(value => [...value].every(char => char.charCodeAt(0) === 9 || char.charCodeAt(0) >= 32 && char.charCodeAt(0) !== 127)) }).strict()).max(32).refine(rows => {
  const keys = rows.map(row => row.name.toLowerCase())
  return keys.every(key => !protectedHeaders.has(key)) && new Set(keys).size === keys.length
})
