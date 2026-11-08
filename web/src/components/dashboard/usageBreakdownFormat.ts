import { t } from '@/i18n'
import { formatMoney } from '@/utils/format'
import type { ExactUsageAmount } from '@/utils/usageRational'

export function formatUsageCost(amount: ExactUsageAmount): string {
  const minor = (amount.numerator * 2n + amount.denominator) / (amount.denominator * 2n)
  const money = (value: bigint) => formatMoney({ currency: 'TXB', minor: value.toString(), display: '' })
  if (amount.numerator > 0n && minor === 0n) return t('usageBreakdown.lessThanAmount', { amount: money(1n) })
  const formatted = money(minor)
  return amount.numerator % amount.denominator === 0n ? formatted : t('usageBreakdown.approximateAmount', { amount: formatted })
}
