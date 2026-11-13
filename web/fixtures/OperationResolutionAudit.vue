<script setup lang="ts">
import { shallowRef } from 'vue'
import type { OperationReceipt, OperationResolution } from '@/api/adminOperations'
import AdminOperationResolutionDialog from '@/components/admin/users/AdminOperationResolutionDialog.vue'
import { t } from '@/i18n'

const params = new globalThis.URLSearchParams(globalThis.location.search)
const now = new Date().toISOString()
const operation: OperationReceipt = { id: 'constructed-receipt', kind: params.has('blocked') ? 'payment_refund' : 'admin_entitlement_edit', status: 'pending_review', createdAt: now, updatedAt: now }
const open = shallowRef(true), busy = shallowRef(false), error = shallowRef<string | null>(null)
const state = { calls: 0, fail: params.has('error'), saved: null as { resolution: OperationResolution; reason: string } | null }
Object.assign(globalThis, { __operationResolutionAudit: state })
async function resolve(payload: { resolution: OperationResolution; reason: string }): Promise<void> {
  state.calls++
  busy.value = true
  error.value = null
  await new Promise(done => globalThis.setTimeout(done, 500))
  if (state.fail) error.value = t('adminUserProfile.reconciliationRequired')
  else { state.saved = payload; open.value = false }
  busy.value = false
}
</script>
<template>
  <UApp><main class="operation-resolution-audit"><UButton :label="$t('adminUserProfile.resolveTitle')" @click="open = true" /><AdminOperationResolutionDialog v-model:open="open" :operation="operation" :busy="busy" :error="error" @resolve="resolve" /></main></UApp>
</template>
<style scoped>.operation-resolution-audit { max-width: 60rem; margin: auto; padding: 1rem; }</style>
