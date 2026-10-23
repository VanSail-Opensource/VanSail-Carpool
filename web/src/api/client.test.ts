import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Purchase, SquadProduct, SquadProductWrite } from './types'
import { api, type AdminPaymentProfile } from './client'

const fetchMock = vi.fn()

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

const squad: SquadProduct = {
  id: 'squad-1',
  groupId: '4fe62019-e890-4d24-9f43-2c22889ec8da',
  name: 'Singapore Plus',
  description: 'Priority Singapore routes.',
  profile: null,
  price: { currency: 'TXB', minor: '725', display: '7.25 TXB' },
  visible: true,
  upstreamPresent: true,
  stockHeldByCurrentUser: false,
  activationRequired: false,
  accessibleNodes: [],
  createdAt: '2026-08-07T00:00:00Z',
  updatedAt: '2026-08-07T00:00:00Z',
}

const entitlement: Purchase = {
  id: 'purchase-1',
  comboId: 'combo-1',
  comboName: 'Everyday',
  price: { currency: 'TXB', minor: '1200', display: '12.00 TXB' },
  grossPrice: { currency: 'TXB', minor: '1200', display: '12.00 TXB' },
  couponDiscount: { currency: 'TXB', minor: '0', display: '0.00 TXB' },
  couponGrantId: null,
  validFrom: '2026-08-07T00:00:00Z',
  validUntil: '2026-09-06T00:00:00Z',
  status: 'cancelled',
  autoRenewEnabled: false,
  trafficLimitBytes: '107374182400',
  resetStrategy: 'MONTH_ROLLING',
  squadUuids: [],
  rolloverMinRemainingBps: 0,
  createdAt: '2026-08-07T00:00:00Z',
  updatedAt: '2026-08-07T00:01:00Z',
}

describe('admin API mutations', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    fetchMock.mockReset()
    vi.unstubAllGlobals()
  })

  it('sends every required squad merchandising field', async () => {
    fetchMock.mockResolvedValue(jsonResponse(squad))
    const payload: SquadProductWrite = {
      groupId: squad.groupId,
      name: squad.name,
      description: squad.description,
      profile: {
        type: 'international_network',
        portMbps: null,
        countryCode: 'SG',
        upstreamCarriers: ['Example Carrier'],
      },
      priceTxbMinor: squad.price.minor,
      visible: squad.visible,
      activationRequired: false,
    }

    await api.updateAdminSquadProduct('squad/1', payload)

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/admin/squad-products/squad%2F1')
    expect(options.method).toBe('PUT')
    expect(JSON.parse(String(options.body))).toEqual(payload)
  })

  it('does not send response-only payment profile fields when saving', async () => {
    const profile: AdminPaymentProfile = {
      id: 'profile-1',
      provider: 'ezpay',
      providerName: 'Primary account',
      enabledChannels: ['alipay'],
      endpoint: 'https://pay.example.test',
      merchantId: 'merchant-1',
      credential: '********',
      acknowledgement: 'ok',
      enabled: true,
      configured: true,
    }
    fetchMock.mockResolvedValue(jsonResponse(profile))

    await api.updateAdminPaymentProfile(profile.id, profile)

    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/admin/payment-profiles/profile-1')
    expect(options.method).toBe('PUT')
    expect(JSON.parse(String(options.body))).toEqual({
      id: profile.id,
      provider: profile.provider,
      providerName: profile.providerName,
      enabledChannels: profile.enabledChannels,
      endpoint: profile.endpoint,
      merchantId: profile.merchantId,
      credential: profile.credential,
      acknowledgement: profile.acknowledgement,
      enabled: profile.enabled,
    })
  })

  it('binds an entitlement cancellation reason to the selected record', async () => {
    fetchMock.mockResolvedValue(jsonResponse(entitlement))

    await api.cancelAdminEntitlement('purchase/1', 'Duplicate account request')

    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/admin/entitlements/purchase%2F1/cancel')
    expect(options.method).toBe('POST')
    expect(JSON.parse(String(options.body))).toEqual({ reason: 'Duplicate account request' })
  })

  it('retries a failed job without sending an invented payload', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }))

    await api.retryAdminJob('job/1')

    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/admin/jobs/job%2F1/retry')
    expect(options.method).toBe('POST')
    expect(options.body).toBeUndefined()
  })

  it('creates a backup without sending an invented payload', async () => {
    fetchMock.mockResolvedValue(jsonResponse({
      id: 'backup-1',
      path: 'backup-1.sqlite',
      sizeBytes: '1024',
      status: 'complete',
      error: '',
      createdAt: '2026-08-07T00:00:00Z',
      completedAt: '2026-08-07T00:00:01Z',
    }))

    await api.createAdminBackup()

    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/admin/backups')
    expect(options.method).toBe('POST')
    expect(options.body).toBeUndefined()
  })

  it('queues manual maintenance with the caller-owned idempotency key', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 'operation-1', kind: 'admin_maintenance', status: 'queued' }, 202))

    await api.runAdminMaintenance('maintenance-attempt-1')

    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/admin/maintenance')
    expect(options.method).toBe('POST')
    expect(new Headers(options.headers).get('Idempotency-Key')).toBe('maintenance-attempt-1')
    expect(options.body).toBeUndefined()
  })

  it('creates and cancels payments with canonical method IDs', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ id: 'payment-1', status: 'pending' }))
      .mockResolvedValueOnce(jsonResponse({ id: 'payment-1', status: 'cancelled' }))

    await api.createPaymentOrder('bepusdt:usdt.trc20', '15000', 'payment-create-1')
    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/payments/orders')
    expect(options.method).toBe('POST')
    expect(new Headers(options.headers).get('Idempotency-Key')).toBe('payment-create-1')
    expect(JSON.parse(String(options.body))).toEqual({ methodId: 'bepusdt:usdt.trc20', txbMinor: '15000' })

    await api.cancelPaymentOrder('payment/1', 'payment-cancel-1')
    const [cancelUrl, cancelOptions] = fetchMock.mock.calls[1] as [string, RequestInit]
    expect(cancelUrl).toBe('/api/v1/payments/orders/payment%2F1/cancel')
    expect(cancelOptions.method).toBe('POST')
    expect(new Headers(cancelOptions.headers).get('Idempotency-Key')).toBe('payment-cancel-1')
    expect(cancelOptions.body).toBeUndefined()
  })

  it('sends the caller-owned purchase idempotency key', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 'purchase-1' }))

    await api.createPurchase('combo-1', ['squad-1'], 'grant-1', 'purchase-attempt-1')

    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/purchases')
    expect(new Headers(options.headers).get('Idempotency-Key')).toBe('purchase-attempt-1')
    expect(JSON.parse(String(options.body))).toEqual({
      comboId: 'combo-1',
      addonSquadProductIds: ['squad-1'],
      couponGrantId: 'grant-1',
    })
  })
})
