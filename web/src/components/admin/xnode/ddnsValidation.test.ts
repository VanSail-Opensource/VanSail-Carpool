import { describe, expect, it } from 'vitest'
import { ddnsErrors, maxDDNSTTLSeconds, type DDNSJob } from './ddnsValidation'
import { validDDNSSettings } from './ddnsForm'
import type { XNodeEndpoint } from '@/api/xnode'

const base: DDNSJob = { id: 'node-ddns', provider: 'cloudflare', zone: 'example.com', name: 'node.example.com', ipv4: '192.0.2.1', ttl_seconds: 300 }
const errors = (changes: Partial<DDNSJob>) => ddnsErrors({ ...base, ...changes })

describe('native DDNS input coverage', () => {
  it.each(['@', 'EXAMPLE.com.', 'node.Example.com.', '*.example.com', '_service._tcp.example.com'])('accepts owned DNS name %s', name => {
    expect(errors({ name })).toEqual({})
  })
  it('canonicalizes IDNA before checking zone ownership', () => {
    expect(errors({ zone: '例子.公司', name: '节点.xn--fsqu00a.xn--55qx5d' })).toEqual({})
    expect(errors({ zone: '例子。公司。', name: '节点．例子｡公司' })).toEqual({})
    expect(errors({ zone: '123', name: 'node.123' })).toEqual({})
  })
  it.each(['node', 'notexample.com', 'example.com.evil.test', 'node..example.com', '*.outside.test', 'https://node.example.com', 'node.example.com ', 'a'.repeat(64) + '.example.com'])('rejects unusable or out-of-zone name %s', name => {
    expect(errors({ name }).name).toBe('ddnsNameInvalid')
  })
  it.each(['example.com/path', 'https://example.com', 'a..com', '*.example.com', ' example.com', '_例子.com'])('rejects invalid DNS zone %s', zone => {
    expect(errors({ zone }).zone).toBe('ddnsZoneInvalid')
  })
  it('bounds TTL before native seconds-to-duration conversion', () => {
    expect(errors({ ttl_seconds: maxDDNSTTLSeconds })).toEqual({})
    for (const value of [-1, 0.5, maxDDNSTTLSeconds + 1, Number.POSITIVE_INFINITY]) expect(errors({ ttl_seconds: value }).ttl_seconds).toBe('ddnsTTLInvalid')
  })
  it('requires at least one address and a discoverable auto source', () => {
    expect(errors({ ipv4: '' }).source).toBe('ddnsSourceRequired')
    expect(errors({ ipv4: 'auto' }).ipv4_url).toBe('ddnsDiscoveryRequired')
    expect(errors({ ipv4: 'auto', interface: 'eth0' })).toEqual({})
    expect(errors({ ipv4: 'auto', ipv4_url: 'https://lookup.example/ip' })).toEqual({})
    expect(errors({ ipv4: 'auto', interface: 'eth 0' }).interface).toBe('ddnsInterfaceInvalid')
  })
  it.each(['http://lookup.example', 'https://user:pass@lookup.example', 'https://lookup.example/#part', 'https://lookup.example/%0a', 'https://lookup.example:0', 'https://lookup.example:'])('rejects invalid HTTPS lookup %s even when an interface is present', ipv4_url => {
    expect(errors({ ipv4: 'auto', interface: 'eth0', ipv4_url }).ipv4_url).toBe('ddnsLookupInvalid')
  })
  it.each(['auto ', 'garbage', '127.0.0.1', '0.0.0.0', '169.254.1.1', '224.0.0.1', '255.255.255.255', '2001:db8::1'])('rejects unusable IPv4 source %s', ipv4 => {
    expect(errors({ ipv4 }).ipv4).toBe('ddnsAddressInvalid')
  })
  it.each(['::', '::1', 'fe80::1', 'ff02::1', '::ffff:192.0.2.1', '192.0.2.1', '2001:db8::1%eth0'])('rejects unusable native IPv6 source %s', ipv6 => {
    expect(errors({ ipv6 }).ipv6).toBe('ddnsAddressInvalid')
  })
  it('preserves native private, documentation and mapped-IPv4 unicast support', () => {
    expect(errors({ ipv4: '10.0.0.1', ipv6: 'fd00::1' })).toEqual({})
    expect(errors({ ipv4: '::ffff:192.0.2.1', ipv6: '2001:db8::1' })).toEqual({})
  })
  it('validates unique jobs and host ownership without requiring DNS names for built-in discovery', () => {
    const job = { ...base, provider: 'builtin:panel', zone: '', name: '', ipv4: 'auto', interface: 'eth0' }
    const hosts = [{ id: 'host-one' }, { id: 'host-two' }] as XNodeEndpoint[]
    expect(ddnsErrors(job)).toEqual({})
    expect(validDDNSSettings({ ddns: [job], ddnsBindings: { [job.id]: { ipv4HostIds: ['host-one'] } } }, hosts)).toBe(true)
    expect(validDDNSSettings({ ddns: [job] }, hosts)).toBe(false)
    expect(validDDNSSettings({ ddns: [job, { ...job }] }, hosts)).toBe(false)
    expect(ddnsErrors({ ...job, ipv4: '192.0.2.1' }).ipv4).toBe('ddnsAddressInvalid')
  })
})
