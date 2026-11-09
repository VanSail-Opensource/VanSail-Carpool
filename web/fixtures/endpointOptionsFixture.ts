import catalog from '../../internal/subscription/tuning_catalog.json'
import { xnodeApi, type XNode, type XNodeConfiguration, type XNodeEndpoint, type XNodeEndpointField, type XNodeEndpointPreview } from '@/api/xnode'
import { ApiError } from '@/api/http'
import { endpointOptionActive } from '@/components/admin/xnode/endpointForm'

type Trace = { calls: string[]; writes: unknown[] }
const definitions = [
  { tag: 'xhttp', protocol: 'vless', transport: 'xhttp', security: 'tls' },
  { tag: 'hysteria', protocol: 'hysteria', transport: 'hysteria', security: 'tls' },
  { tag: 'ws', protocol: 'vless', transport: 'ws', security: 'tls' },
  { tag: 'grpc', protocol: 'trojan', transport: 'grpc', security: 'tls' },
  { tag: 'httpupgrade', protocol: 'vmess', transport: 'httpupgrade', security: 'tls' },
  { tag: 'reality', protocol: 'vless', transport: 'tcp', security: 'reality' },
]
export function endpointOptionsFixture(params: URLSearchParams, trace: Trace, nodes: XNode[], configurations: XNodeConfiguration[]) {
  if (!params.has('endpoint-options')) return
  const config = configurations[0]
  if (config) config.document.inbounds = definitions.map(item => ({ tag: item.tag, port: 443, protocol: item.protocol, settings: {}, streamSettings: { network: item.transport, security: item.security } }))
  const preferred = definitions.find(item => item.tag === params.get('endpoint-protocol')) ?? definitions[0]!
  let items: XNodeEndpoint[] = params.has('endpoint-empty') ? [] : nodes.slice(0, 1).map(node => ({ id: 'endpoint-one', nodeId: node.id, inboundTag: preferred.tag, name: 'Client endpoint', address: 'client.example', port: 443, enabled: true, hidden: false, metadata: { protocol: preferred.protocol, transport: preferred.transport, security: preferred.security }, overrides: { legacyHost: { retainedFixture: true }, clientOptions: preferred.tag === 'hysteria' ? { hysteriaPorts: [{ min: 20000, max: 30000 }], hysteriaHopInterval: { min: 5, max: 10 } } : preferred.tag === 'xhttp' ? { xhttpPath: '/client', xhttpXPaddingBytes: { min: 100, max: 1000 }, downloadEnabled: true, downloadAddress: 'download.example', downloadPort: 443, downloadSecurity: 'tls' } : {} } }))
  xnodeApi.endpoints = async () => ({ items: structuredClone(items) })
  let failed = false
  xnodeApi.previewEndpoint = async (draft, signal) => {
    trace.calls.push('preview-endpoint:' + String(draft.inboundTag))
    if (params.has('endpoint-preview-slow')) await new Promise(resolve => setTimeout(resolve, 1500))
    if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
    if (params.has('endpoint-preview-error') && !failed) { failed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
    const metadata = definitions.find(item => item.tag === draft.inboundTag)
    const values = draft.overrides?.clientOptions ?? {}
    const fields = !metadata ? [] : catalog.filter(field => {
      if (field.ownership === 'member') return false
      if (field.key.startsWith('download')) {
        if (metadata.transport !== 'xhttp') return false
        const security = values.downloadSecurity ?? metadata.security
        if (field.key.startsWith('downloadTls') && field.key !== 'downloadTlsServerName') return security === 'tls'
        if (field.key.startsWith('downloadReality') || field.key === 'downloadSpiderX') return security === 'reality'
        if (field.key === 'downloadFingerprint' || field.key === 'downloadTlsServerName') return security === 'tls' || security === 'reality'
        return true
      }
      if (field.context === 'tls') return ['tls', 'reality'].includes(metadata.security)
      if (field.context === 'tls-only') return metadata.security === 'tls'
      if (field.context === 'reality') return metadata.security === 'reality'
      return field.context === metadata.transport
    }).map(field => {
      const { key, group, kind, minimum, maximum, unit, advanced, context, ownership } = field
      const enumeration = key.includes('Fingerprint') || key === 'fingerprint' ? ['chrome', 'firefox', 'safari', 'random', 'unsafe'] : field.enum
      const dependencies: NonNullable<XNodeEndpointField['dependencies']> = ('dependencies' in field ? field.dependencies ?? [] : []).map(rule => ({ ...rule, inheritedValues: rule.key === 'xhttpMode' ? [String(values.xhttpMode ?? 'auto')] : undefined }))
      if (key.startsWith('download') && key !== 'downloadEnabled') dependencies.push({ key: 'downloadEnabled', notValues: ['false'] })
      const restrictions = ['tlsAllowInsecure', 'downloadTlsAllowInsecure'].includes(key) ? ['fixedOff'] : []
      return { key, group, kind, minimum, maximum, unit, advanced, context, ownership, enum: enumeration, available: true, dependencies, restrictions } as XNodeEndpointField
    })
    const fieldErrors: XNodeEndpointPreview['fieldErrors'] = []
    for (const key of ['name', 'address', 'nodeId', 'inboundTag'] as const) if (!draft[key]) fieldErrors.push({ field: key, code: 'required' })
    if (!metadata && draft.inboundTag) fieldErrors.push({ field: 'inboundTag', code: 'binding' })
    if (!draft.port || draft.port < 1 || draft.port > 65535) fieldErrors.push({ field: 'port', code: 'invalid' })
    if (params.has('endpoint-validation-error')) fieldErrors.push({ field: 'clientOptions', code: 'xrayValidation' })
    const clients = (['xray', 'sing-box', 'clash', 'stash', 'native', 'base64'] as const).map(format => {
      const unsupported = format === 'sing-box' && metadata?.transport === 'xhttp' || format === 'stash' && metadata?.transport === 'httpupgrade'
      const omittedOptions = fields.filter(field => field.key in values && endpointOptionActive(field, values)).filter(field => {
        if (field.key === 'hysteriaHopInterval') {
          const interval = values.hysteriaHopInterval
          return format === 'native' || format === 'base64' || format === 'stash' && interval?.min !== interval?.max
        }
        const spec = catalog.find(item => item.key === field.key)
        return spec && !('essential' in spec && spec.essential) && !(spec.mappings as Record<string, string>)[format]
      }).map(field => field.key)
      return { format, supported: !unsupported, reasons: unsupported ? ['transport'] : [], omittedOptions }
    })
    return { valid: !fieldErrors.length, metadata: metadata ? { protocol: metadata.protocol, transport: metadata.transport, security: metadata.security } : { protocol: '', transport: '', security: '' }, inbounds: draft.nodeId ? definitions : [], fields, fieldErrors, clients }
  }
  xnodeApi.saveEndpoint = async draft => {
    trace.calls.push('save-endpoint')
    if (params.has('endpoint-save-error')) throw new ApiError(422, { code: 'XNODE_INPUT_INVALID', message: '' })
    trace.writes.push(structuredClone(draft))
    const selected = definitions.find(item => item.tag === draft.inboundTag)
    const item = { ...draft, id: draft.id || 'endpoint-created', metadata: selected ? { protocol: selected.protocol, transport: selected.transport, security: selected.security } : undefined } as XNodeEndpoint
    items = [...items.filter(value => value.id !== item.id), item]
    return structuredClone(item)
  }
}
