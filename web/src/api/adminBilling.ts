import type { components } from './generated'
import type { BillingAmountLimits } from './types'
import { request } from './http'

type BillingAmountLimitsWrite = components['schemas']['BillingAmountLimitsWrite']

export const adminBillingApi = {
  getAmountLimits: () => request<BillingAmountLimits>('/api/v1/admin/billing/amount-limits'),
  updateAmountLimits: (minimumTxbMinor: string, maximumTxbMinor: string) =>
    request<BillingAmountLimits>('/api/v1/admin/billing/amount-limits', {
      method: 'PUT',
      body: { minimumTxbMinor, maximumTxbMinor } satisfies BillingAmountLimitsWrite,
    }),
}
