import { readFileSync } from 'node:fs'
import { URL } from 'node:url'
import { describe, expect, it } from 'vitest'
import { parse } from 'yaml'
import { z } from 'zod'

type Document = Record<string, unknown>
type JSONSchema = Parameters<typeof z.fromJSONSchema>[0]
const rootURL = new URL('../../../api/openapi.yaml', import.meta.url)
const documents = new Map<string, Document>()

function readDocument(url: URL): Document {
  const cached = documents.get(url.href)
  if (cached) return cached
  const document = parse(readFileSync(url, 'utf8'), { merge: true }) as Document
  documents.set(url.href, document)
  return document
}

function at(value: unknown, ...keys: string[]): unknown {
  return keys.reduce((node, key) => (node as Document)[key], value)
}

function resolveReference(reference: string, base: URL): { value: unknown; url: URL } {
  const url = new URL(reference, base)
  const pointer = url.hash.slice(1)
  url.hash = ''
  const keys = pointer ? pointer.slice(1).split('/').map((key) => key.replace(/~1/g, '/').replace(/~0/g, '~')) : []
  return { value: at(readDocument(url), ...keys), url }
}

// Only the bounded, nonrecursive schemas exercised below are expanded.
function expand(value: unknown, base: URL): unknown {
  if (Array.isArray(value)) return value.map((child) => expand(child, base))
  if (typeof value !== 'object' || value === null) return value
  const node = value as Document
  if (typeof node.$ref === 'string') {
    const resolved = resolveReference(node.$ref, base)
    return expand(resolved.value, resolved.url)
  }
  return Object.fromEntries(Object.entries(node).map(([key, child]) => [key, expand(child, base)]))
}

function schema(shard: string, name: string): JSONSchema {
  const url = new URL('components/schemas/' + shard, rootURL)
  return expand(readDocument(url)[name], url) as JSONSchema
}

function requestSchema(shard: string, method = 'post'): Document {
  const url = new URL('paths/' + shard, rootURL)
  return expand(at(readDocument(url), method, 'requestBody', 'content', 'application/json', 'schema'), url) as Document
}

function accepts(document: unknown, value: unknown): boolean {
  return z.fromJSONSchema(document as JSONSchema).safeParse(value).success
}

describe('OpenAPI input and recovery boundaries', () => {
  it('accepts signed and capability-only BEP callbacks under JSON Schema union semantics', () => {
    const contract = requestSchema('api-v1-webhooks-bepusdt-capability.yaml')
    const alternatives = (contract.anyOf ?? contract.oneOf) as JSONSchema[]
    const acceptsCallback = (value: unknown): boolean => {
      const count = alternatives.filter((branch) => accepts(branch, value)).length
      return contract.oneOf ? count === 1 : count > 0
    }
    const unsigned = { order_id: 'order-1', amount: '3.00', actual_amount: 3, token: 'USDT', status: 2 }
    const signed = { ...unsigned, trade_id: 'trade-1', signature: 'provider-signature' }

    // Signed data also satisfies the unsigned shape because providers may add fields.
    expect(alternatives.filter((branch) => accepts(branch, signed))).toHaveLength(2)
    expect(acceptsCallback(unsigned)).toBe(true)
    expect(acceptsCallback(signed)).toBe(true)
    expect(acceptsCallback({ ...unsigned, amount: undefined })).toBe(false)
  })

  it('keeps each profiled payment rail with its actual provider', () => {
    const contract = schema('schemas-04.yaml', 'PaymentMethodID')
    for (const value of ['ezpay:account:alipay', 'ezpay:account:wxpay', 'ezpay:account:wechat', 'bepusdt:account:usdt.trc20', 'bepusdt:account:usdc.base']) {
      expect(accepts(contract, value), value).toBe(true)
    }
    for (const value of ['ezpay:account:usdt.trc20', 'bepusdt:account:alipay', 'ezpay::alipay', 'bepusdt:account:usdc.ton']) {
      expect(accepts(contract, value), value).toBe(false)
    }
  })

  it('accepts exact decimal rewards and the existing bounded affiliate amounts', () => {
    const reward = schema('schemas-06.yaml', 'GroupMessageRewardStatus')
    const value = { enabled: true, localDate: '2026-11-12', messageCount: 3, threshold: 3, rewardMinor: '125', rewarded: false }
    expect(accepts(reward, value)).toBe(true)
    expect(accepts(reward, { ...value, rewardMinor: '1.25' })).toBe(false)
    expect(accepts(reward, { ...value, rewardMinor: String.fromCharCode(92) + 'ddd' })).toBe(false)
    const affiliate = schema('schemas-20.yaml', 'AffiliateTXBReward')
    expect(accepts(affiliate, { kind: 'txb', txbMinor: 100_000_000_000 })).toBe(true)
    expect(accepts(affiliate, { kind: 'txb', txbMinor: 100_000_000_001 })).toBe(false)
  })

  it('accepts missing country metadata while rejecting malformed country labels', () => {
    for (const [shard, name] of [['schemas-02.yaml', 'CatalogNode'], ['schemas-03.yaml', 'ProxyNode']] as const) {
      const country = at(schema(shard, name), 'properties', 'countryCode')
      expect(accepts(country, '')).toBe(true)
      expect(accepts(country, 'US')).toBe(true)
      expect(accepts(country, 'USA')).toBe(false)
    }
  })

  it('preserves null inheritance and explicit empty currency switches', () => {
    const currencies = at(schema('usdt.yaml', 'USDTNetworkConfig'), 'properties', 'currencies')
    expect(accepts(currencies, null)).toBe(true)
    expect(accepts(currencies, [])).toBe(true)
    expect(accepts(currencies, ['USDT', 'USDC'])).toBe(true)
    expect(accepts(currencies, ['BTC'])).toBe(false)
  })

  it('accepts the documented automatic-whitelist partial body', () => {
    const contract = requestSchema('api-v1-admin-xnode-global.yaml', 'put')
    const settings = { enabled: true, durationHours: 24 }
    expect(accepts(contract, { automaticWhitelist: settings })).toBe(true)
    expect(accepts(contract, {})).toBe(false)
  })

  it('declares browser selection and excludes admin tokens from payment-owner receipts', () => {
    const routeURL = new URL('paths/api-v1-payments-return-provider-orderid-status.yaml', rootURL)
    const route = readDocument(routeURL)
    const security = at(route, 'get', 'security') as Document[]
    expect(security.some((alternative) => 'panelApiToken' in alternative)).toBe(false)
    expect(security.some((alternative) => 'checkoutReturnCapability' in alternative)).toBe(true)
    const parameter = expand((route.parameters as unknown[])[0], routeURL) as Document
    expect(parameter).toMatchObject({ name: 'X-TXC-Session-Scope', in: 'header', schema: { enum: ['browser'] } })
    expect(at(route, 'get', 'responses', '500')).toBeDefined()
    const order = readDocument(new URL('paths/api-v1-payments-orders-id.yaml', rootURL))
    expect(at(order, 'get', 'responses', '500')).toBeDefined()
    expect(at(order, 'get', 'responses', '404')).toBeDefined()
  })

  it('gives every registered response an OpenAPI description', () => {
    const paths = readDocument(rootURL).paths as Document
    for (const [route, entry] of Object.entries(paths)) {
      const resolved = resolveReference((entry as { $ref: string }).$ref, rootURL)
      for (const [method, operation] of Object.entries(resolved.value as Document)) {
        if (!['get', 'post', 'put', 'patch', 'delete'].includes(method)) continue
        for (const [status, response] of Object.entries((operation as { responses: Document }).responses)) {
          const value = response as Document
          const effective = typeof value.$ref === 'string' ? resolveReference(value.$ref, resolved.url).value as Document : value
          expect(effective.description, route + ' ' + method + ' ' + status).toBeTypeOf('string')
        }
      }
    }
  })
})
