import { t } from '@/i18n'
import { formatMemberMoney } from './displayCurrency'

export function pricingMoney(value: unknown): string {
  const minor = String(value ?? '')
  return /^-?\d+$/.test(minor) ? formatMemberMoney({ currency: 'TXB', minor, display: '' }) : t('common.notAvailable')
}

export function pricingLabel(kind: 'states' | 'reasons', value: unknown): string {
  const key = `pricing.${kind}.${String(value ?? '').replace(kind === 'reasons' ? /^PRICING_/ : /^$/, '')}`
  const label = t(key)
  if (label !== key) return label
  if (kind === 'reasons' && value && !/^[A-Z][A-Z0-9_]+$/.test(String(value))) return String(value)
  return t(kind === 'states' ? 'common.notAvailable' : 'pricing.reasons.OTHER')
}
