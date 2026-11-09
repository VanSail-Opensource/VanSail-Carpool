import { xnodeApi, type XNode } from '@/api/xnode'
import { ApiError } from '@/api/http'
import { adminOperationsApi } from '@/api/adminOperations'
import type { OperationReceipt } from '@/api/types'

export function certificateFixture(params: URLSearchParams, trace: { writes: unknown[]; calls: string[] }, nodes: XNode[]): void {
  xnodeApi.certificateOperation = async () => ({ receipt: null, result: {} })
  if (!params.has('certificates')) return
  const operations = new Map<string, { receipt: OperationReceipt; action: string; revision: number }>()
  const profiles = new Map<string, string>()
  const oldNode = xnodeApi.node, oldMetrics = xnodeApi.metrics, oldCommand = xnodeApi.command, oldOperation = adminOperationsApi.getOperation, oldGlobal = xnodeApi.global
  let recoveryFailed = false, ambiguous = false
  const stamp = new Date().toISOString()
  xnodeApi.global = async signal => {
    if (params.has('certificate-providers-error')) { trace.calls.push('certificate-providers-error'); throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
    const value = await oldGlobal(signal); return { ...value, providers: { ...value.providers, cloudflare: { type: 'cloudflare', credentials: {} } } }
  }
  xnodeApi.node = async (id, signal) => {
    const value = await oldNode(id, signal)
    value.settings.certificates = {
      primary: { provider: params.has('certificate-dns') && !params.has('certificate-inline') ? 'cloudflare' : '', config: params.has('certificate-file') || params.has('certificate-relative-file') ? { cert_mode: 'file', cert_file: params.has('certificate-relative-file') ? './tls/cert.pem' : '/etc/x-node/tls/cert.pem', key_file: params.has('certificate-relative-file') ? '../shared/key.pem' : '/etc/x-node/tls/key.pem' }
        : params.has('certificate-content') ? { cert_mode: 'content', domain: 'proxy.example', cert_content: '', key_content: '' }
        : params.has('certificate-inline') ? { cert_mode: 'dns', domain: 'proxy.example', dns_provider: 'cloudflare', dns_env: { CLOUDFLARE_DNS_API_TOKEN: 'synthetic-fixture-credential' }, auto_tls: true }
        : { cert_mode: params.has('certificate-dns') ? 'dns' : 'http', domain: 'proxy.example', email: 'certificates@proxy.example', auto_tls: true } },
      failed: { config: { cert_mode: 'http', domain: 'pending.proxy.example', auto_tls: true } },
    }
    value.settings.inboundCertificates = { paid: 'primary' }
    const primary = value.settings.certificates.primary!
    if (params.has('certificate-inferred-file')) primary.config = { cert_mode: '', auto_tls: false, cert_file: './tls/cert.pem', key_file: '../shared/key.pem' }
    if (params.has('certificate-content-no-domain') || params.has('certificate-inferred-content') || params.has('certificate-content-wildcard')) primary.config = { cert_mode: params.has('certificate-inferred-content') ? '' : 'content', domain: params.has('certificate-content-wildcard') ? '*.example.com' : '', cert_content: '-----BEGIN CERTIFICATE-----\nQUJD\n-----END CERTIFICATE-----', key_content: '-----BEGIN PRIVATE KEY-----\nQUJD\n-----END PRIVATE KEY-----' }
    if (params.has('certificate-inferred-http')) primary.config = { cert_mode: '', auto_tls: true, domain: 'proxy.example' }
    if (params.has('certificate-normalized-mode')) primary.config = { cert_mode: ' HTTP ', domain: 'proxy.example' }
    if (params.has('certificate-idn')) primary.config.domain = 'b\u00fccher.example.'
    if (params.has('certificate-single-label')) primary.config.domain = 'proxy'
    if (params.has('certificate-self-name')) primary.config = { cert_mode: 'self', domain: 'Private service name' }
    return value
  }
  xnodeApi.metrics = async signal => {
    const value = await oldMetrics(signal)
    value.nodes = value.nodes.map(node => ({ ...node,
      capabilities: [...(node.capabilities ?? []), ...(params.has('certificate-old-agent') ? [] : ['certificate_lifecycle_v1'])],
      certificates: {
        primary: { mode: 'http', phase: 'idle', active: true, domain: 'proxy.example', sans: ['proxy.example', 'cdn.proxy.example'], issuer: 'VanSail fixture issuer', not_before: new Date(Date.now() - 86400000).toISOString(),
          expires: new Date(Date.now() + (params.has('certificate-expired') ? -1 : 10) * 86400000).toISOString(), expiring: true, fingerprint: 'ab'.repeat(32), acquired_fingerprint: 'ab'.repeat(32), revision: 3 },
        failed: { mode: 'http', phase: 'failed', active: false, domain: 'pending.proxy.example', sans: [], issuer: '', error_code: 'CERTIFICATE_ACME_FAILED', revision: 3 },
      },
    }))
    return value
  }
  if (params.has('certificate-pending') && nodes[0]) {
    const receipt: OperationReceipt = { id: 'certificate-recovered', kind: 'xnode_command', status: 'pending_review', createdAt: stamp, updatedAt: stamp }
    operations.set(receipt.id, { receipt, action: 'renew', revision: 3 }); profiles.set('primary', receipt.id)
  }
  xnodeApi.certificateOperation = async (_node, profile) => {
    trace.calls.push('certificate-operation:' + profile)
    if (params.has('certificate-recovery-error') && !recoveryFailed) { recoveryFailed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
    const value = operations.get(profiles.get(profile) ?? '')
    return value ? { receipt: { ...value.receipt }, result: { action: value.action, expected_revision: value.revision } } : { receipt: null, result: {} }
  }
  xnodeApi.command = async (node, body, key) => {
    if (body.name !== 'certificates.run') return oldCommand(node, body, key)
    trace.writes.push({ certificateCommand: { node, body, key } })
    const existing = operations.get(key ?? '')
    if (existing) return { ...existing.receipt }
    const receipt: OperationReceipt = { id: key!, kind: 'xnode_command', status: 'queued', createdAt: stamp, updatedAt: stamp }
    if (!(params.has('certificate-replay') && !ambiguous)) {
      operations.set(receipt.id, { receipt, action: body.action!, revision: body.expectedRevision! }); profiles.set(body.profileId!, receipt.id)
    }
    if ((params.has('certificate-ambiguous') || params.has('certificate-replay')) && !ambiguous) { ambiguous = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
    return { ...receipt }
  }
  adminOperationsApi.getOperation = async id => {
    const value = operations.get(id)
    if (!value) return oldOperation(id)
    value.receipt = { ...value.receipt, status: params.has('certificate-failure') ? 'failed' : params.has('certificate-pending') ? 'pending_review' : 'succeeded', updatedAt: new Date().toISOString() }
    return { ...value.receipt }
  }
}
