// HourlyOffer.requiredHours is newer than the generated client type. Read it
// through an optional cast so the page keeps working before types regenerate:
// 0 (or a missing value) inherits the account-wide activationHours.
export function hourlyRequiredHours(offer: object, fallback: number): number {
  const value = (offer as { requiredHours?: unknown }).requiredHours
  return typeof value === 'number' && Number.isInteger(value) && value > 0 ? value : fallback
}
