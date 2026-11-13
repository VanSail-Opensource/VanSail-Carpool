import type { OperationReceipt } from '@/api/adminOperations'

const repairableKinds = new Set([
  'admin_entitlement_edit', 'admin_entitlement_refund', 'admin_combo_replacement',
  'admin_bulk_extension', 'node_compensation',
])

// Only the server can validate retained item evidence; this is an entrance gate.
export function canRepairOperation(operation: Pick<OperationReceipt, 'kind' | 'status'> | null): boolean {
  return Boolean(operation && repairableKinds.has(operation.kind)
    && (operation.status === 'pending_review' || operation.status === 'partial'))
}
