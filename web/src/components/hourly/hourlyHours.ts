import type { HourlyOffer, HourlySquad } from '@/api/hourly'

// Zero inherits the account-wide activationHours.
export function hourlyRequiredHours(offer: Pick<HourlyOffer, 'requiredHours'>, fallback: number): number {
  const value = offer.requiredHours
  return typeof value === 'number' && Number.isInteger(value) && value > 0 ? value : fallback
}

// Keep the confirmation in native TXB minor units. An absent or inconsistent
// server quote must never become a zero-cost activation promise.
export function hourlyActivationQuote(item: HourlySquad, fallback: number) {
  const hours = hourlyRequiredHours(item.offer, fallback)
  const amount = item.requiredBalance
  const rate = item.offer.hourlyPrice
  if (!Number.isInteger(hours) || hours < 1 || hours > 2_562_047 ||
    amount?.currency !== 'TXB' || rate.currency !== 'TXB' ||
    !/^[1-9]\d*$/.test(amount.minor) || !/^[1-9]\d*$/.test(rate.minor) ||
    BigInt(amount.minor) !== BigInt(rate.minor) * BigInt(hours)) return null
  return { hours, minor: amount.minor, rateMinor: rate.minor }
}
