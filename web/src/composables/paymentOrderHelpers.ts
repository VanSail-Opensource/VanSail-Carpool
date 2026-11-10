import QRCode from 'qrcode'

import type { FeaturePaymentMethod, FeaturePaymentOrder } from '@/api/features'

export function isTerminalPaymentStatus(status: FeaturePaymentOrder['status']): boolean {
  return ['paid', 'cancelled', 'expired', 'failed', 'refunded'].includes(status)
}

export function positivePaymentMinor(value: string | undefined, fallback: bigint): bigint {
  return value && /^\d+$/.test(value) && BigInt(value) > 0n ? BigInt(value) : fallback
}

export function resolveReissueCandidate(
  candidate: FeaturePaymentOrder,
  methods: readonly FeaturePaymentMethod[],
  minimumMinor: bigint,
  maximumMinor: bigint,
): { amountMinor: string, methodId: string } | null {
  if (candidate.status !== 'expired' && candidate.status !== 'failed') return null
  return resolveOrderSelection(candidate, methods, minimumMinor, maximumMinor)
}

export function resolveOrderSelection(
  candidate: FeaturePaymentOrder,
  methods: readonly FeaturePaymentMethod[],
  minimumMinor: bigint,
  maximumMinor: bigint,
): { amountMinor: string, methodId: string } | null {
  const method = methods.find((item) => item.id === candidate.methodId && item.available)
  const minor = candidate.txb.currency === 'TXB' ? candidate.txb.minor : ''
  if (
    !method || method.provider !== candidate.provider || method.rail !== candidate.providerRail
    || !/^\d+$/.test(minor) || BigInt(minor) < minimumMinor || BigInt(minor) > maximumMinor
  ) return null
  return { amountMinor: minor, methodId: method.id }
}

export function paymentQrDataUrl(order: FeaturePaymentOrder): Promise<string | null> {
  const payload = order.receivingAddress ?? order.qrPayload
  if (!payload) return Promise.resolve(null)
  return QRCode.toDataURL(payload, {
    width: 360,
    margin: 1,
    color: { dark: '#111512', light: '#edf2ee' },
    errorCorrectionLevel: 'M',
  })
}
