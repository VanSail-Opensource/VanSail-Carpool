import type { HourlyOffer } from '@/api/hourly'

// Zero inherits the account-wide activationHours.
export function hourlyRequiredHours(offer: Pick<HourlyOffer, 'requiredHours'>, fallback: number): number {
  const value = offer.requiredHours
  return typeof value === 'number' && Number.isInteger(value) && value > 0 ? value : fallback
}
