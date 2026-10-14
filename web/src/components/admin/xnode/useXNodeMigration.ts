import { computed, onMounted, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeApi, type XNodeMigration, type XNodeMigrationPreview, type XNodeMigrationPreviewWrite } from '@/api/xnode'
import type { OperationReceipt } from '@/api/types'
import { useOperationReceipt, operationIsActive } from '@/composables/useOperationReceipt'
import { localizedError } from '@/i18n'

export function useXNodeMigration() {
  const state = shallowRef<XNodeMigration | null>(null)
  const preview = shallowRef<XNodeMigrationPreview | null>(null)
  const loading = shallowRef(true)
  const saving = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const operation = useOperationReceipt()
  const busy = computed(() => saving.value || operationIsActive(operation.receipt.value?.status))
  let active = true
  let generation = 0
  async function load(): Promise<void> {
    const current = ++generation
    loading.value = true
    try { const value = await xnodeApi.migration(); if (active && current === generation) { state.value = value; error.value = null } }
    catch (caught) { if (active && current === generation) error.value = localizedError(caught, 'xnode.migration.loadFailed') }
    finally { if (active && current === generation) loading.value = false }
  }
  async function capture(input: XNodeMigrationPreviewWrite): Promise<boolean> {
    if (busy.value) return false
    saving.value = true; error.value = null
    try {
      const value = await xnodeApi.previewMigration(input)
      if (!active) return false
      preview.value = value; await load(); return true
    } catch (caught) { if (active) error.value = localizedError(caught, 'xnode.migration.actionFailed'); return false }
    finally { if (active) saving.value = false }
  }
  async function run(action: () => Promise<OperationReceipt>): Promise<void> {
    if (busy.value) return
    saving.value = true; error.value = null
    try { const value = await action(); if (active) operation.track(value) }
    catch (caught) { if (active) error.value = localizedError(caught, 'xnode.migration.actionFailed') }
    finally { if (active) saving.value = false }
  }
  watch(operation.terminal, value => { if (value) void load() })
  onMounted(() => void load())
  onScopeDispose(() => { active = false; generation++ })
  return { state, preview, loading, error, busy, saving, operation, load, capture, run }
}
