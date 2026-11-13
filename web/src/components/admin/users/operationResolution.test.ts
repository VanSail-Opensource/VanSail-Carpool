import { describe, expect, it } from 'vitest'
import { canRepairOperation } from './operationResolution'

describe('administrative receipt repair entrance', () => {
  it.each(['admin_entitlement_edit', 'admin_entitlement_refund', 'admin_combo_replacement', 'admin_bulk_extension', 'node_compensation'])('allows only reviewable parent receipts for %s', kind => {
    expect(canRepairOperation({ kind, status: 'pending_review' })).toBe(true)
    expect(canRepairOperation({ kind, status: 'partial' })).toBe(true)
    expect(canRepairOperation({ kind, status: 'processing' })).toBe(false)
    expect(canRepairOperation({ kind, status: 'succeeded' })).toBe(false)
  })
  it.each(['payment_refund', 'payment_cancel', 'purchase_commit', 'hourly_enable', 'emby_setup', 'xnode_command', 'admin_temporary_ban', 'unknown_future_kind'])('keeps domain reconciliation outside generic repair for %s', kind => {
    expect(canRepairOperation({ kind, status: 'pending_review' })).toBe(false)
    expect(canRepairOperation({ kind, status: 'partial' })).toBe(false)
  })
})
