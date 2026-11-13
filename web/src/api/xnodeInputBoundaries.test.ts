import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { parse } from 'yaml'
import { z } from 'zod'

const schemas = parse(readFileSync(resolve(process.cwd(), '../api/components/schemas/xnode.yaml'), 'utf8'))
const node = z.fromJSONSchema(schemas.XNodeNodeWrite)
const settings = z.fromJSONSchema(schemas.XNodeSettings)

describe('current native node write boundaries', () => {
  it('accepts six editable fields and rejects retired or server-owned node inputs', () => {
    const value = { id: 'node', name: 'Amsterdam', enabled: true, configId: 'config', countryCode: 'NL', providerName: '' }
    expect(node.safeParse(value).success).toBe(true)
    for (const field of ['multiplier', 'agentId', 'desiredRevision', 'acceptedRevision', 'appliedRevision', 'createdAt', 'updatedAt', 'retiredAt', 'removingAt']) {
      expect(node.safeParse({ ...value, [field]: 1 }).success, field).toBe(false)
    }
  })
  it('accepts redirect-only DNS and rejects retired public migration fields', () => {
    expect(settings.safeParse({ dns: { redirect: true } }).success).toBe(true)
    expect(settings.safeParse({ dns: {} }).success).toBe(true)
    expect(settings.safeParse({ legacyDnsLoopback: true }).success).toBe(false)
    for (const dns of [{ mode: 'xray' }, { upstreams: ['1.1.1.1'] }]) expect(settings.safeParse({ dns }).success).toBe(false)
  })
  it('accepts plural DDNS bindings and rejects retired singular aliases', () => {
    expect(settings.safeParse({ ddnsBindings: { job: { ipv4HostIds: ['host'], ipv6HostIds: [] } } }).success).toBe(true)
    for (const field of ['ipv4HostId', 'ipv6HostId']) expect(settings.safeParse({ ddnsBindings: { job: { [field]: 'host' } } }).success).toBe(false)
  })
  it('rejects the ignored active-IP telemetry flag while preserving sampling controls', () => {
    const telemetry = z.fromJSONSchema(schemas.XNodeGlobal.properties.telemetry)
    expect(telemetry.safeParse({ sample_seconds: 1, report_seconds: 5 }).success).toBe(true)
    expect(telemetry.safeParse({ include_active_ips: false }).success).toBe(false)
  })
  it('bounds DDNS TTL and interval without rejecting uppercase HTTPS schemes', () => {
    const ddns = { id: 'job', provider: 'cloudflare', zone: 'example.com', name: '@', ipv4: 'auto', ipv4_url: 'HTTPS://lookup.example/ip', ttl_seconds: 2147483647, interval_seconds: 86400 }
    expect(settings.safeParse({ ddns: [ddns] }).success).toBe(true)
    for (const values of [{ ttl_seconds: 2147483648 }, { ttl_seconds: -1 }, { interval_seconds: -1 }, { interval_seconds: 86401 }]) {
      expect(settings.safeParse({ ddns: [{ ...ddns, ...values }] }).success).toBe(false)
    }
  })
})
