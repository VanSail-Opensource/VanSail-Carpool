import type { DIYField, DIYOverrides, DIYPreview } from '@/api/xnodeMember'

// Opt-in visual fixture only. This is not server validation and is never imported
// by the application. The default fixture still uses the pinned Go helper.
const fields: DIYField[] = [
  { key: 'fingerprint', group: 'tls', kind: 'enum', enum: ['chrome', 'firefox', 'safari'], minimum: 0, maximum: 0, unit: '', advanced: false },
  { key: 'curvePreferences', group: 'tls', kind: 'strings', enum: ['X25519', 'CurveP256', 'X25519MLKEM768'], minimum: 0, maximum: 0, unit: '', advanced: true },
  { key: 'ssUDPOverTCP', group: 'shadowsocks', kind: 'boolean', minimum: 0, maximum: 0, unit: '', advanced: false },
  { key: 'muxXudpConcurrency', group: 'mux', kind: 'integer', minimum: -1, maximum: 1024, unit: 'count', advanced: true },
  { key: 'xhttpMode', group: 'xhttp', kind: 'enum', enum: ['auto', 'packet-up', 'stream-up', 'stream-one'], minimum: 0, maximum: 0, unit: '', advanced: false },
  { key: 'xhttpXPaddingBytes', group: 'xhttp', kind: 'range', minimum: 0, maximum: 65536, unit: 'bytes', advanced: true },
  { key: 'xhttpHeaders', group: 'xhttp', kind: 'headers', minimum: 0, maximum: 0, unit: '', advanced: true },
  { key: 'xmuxMaxConcurrency', group: 'xmux', kind: 'range', minimum: 0, maximum: 1024, unit: 'count', advanced: true },
]

export function localDIYPreview(overrides: DIYOverrides, empty: boolean): DIYPreview & { fields: DIYField[] } {
  const visible = empty ? [] : fields
  return {
    valid: true, fieldErrors: {}, paths: [], fields: structuredClone(visible),
    compatibility: ['xray', 'sing-box', 'clash', 'stash', 'native', 'base64'].flatMap(format => visible.map(field => {
      const customized = field.key in overrides
      const unsupported = customized && format === 'base64'
      return { key: field.key, format, state: unsupported ? 'unsupported' as const : customized ? 'applied' as const : 'inherited' as const, appliedHosts: customized && !unsupported ? 2 : 0, incompatibleHosts: unsupported ? 2 : 0, reasons: unsupported ? ['format'] : [] }
    })),
  }
}
