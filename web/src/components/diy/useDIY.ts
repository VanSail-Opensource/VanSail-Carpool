import { computed, onMounted, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { xnodeMemberApi, type DIYView, type DIYOverrides, type DIYWrite, type DIYPreview, type RoutingChoice } from '@/api/xnodeMember'
import { memberOperationsApi } from '@/api/memberOperations'
import { useOperationReceipt } from '@/composables/useOperationReceipt'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { createLatestRequest } from '@/utils/latestRequest'
import { diySchema } from './diyValidation'
import { useDIYPreview } from './useDIYPreview'

export function useDIY() {
  const view = shallowRef<DIYView | null>(null)
  const overrides = ref<DIYOverrides>({}), choices = ref<RoutingChoice[]>([])
  const preview = shallowRef<DIYPreview | null>(null)
  const fieldErrors = shallowRef<Record<string, string>>({})
  const loading = shallowRef(true), submitting = shallowRef(false), saved = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const latest = createLatestRequest(), keys = new Map<string, string>()
  let controller: InstanceType<typeof globalThis.AbortController> | undefined
  let validator: InstanceType<typeof globalThis.AbortController> | undefined
  let disposed = false, hydrated = false
  const tracker = useOperationReceipt(1500, async id => {
    const receipt = await memberOperationsApi.getOperation(id)
    if (!disposed && !['queued', 'processing'].includes(receipt.status)) void load(true)
    return receipt
  })
  const autoPreview = useDIYPreview(() => view.value ? { settingsRevision: view.value.settingsRevision, routingRevision: view.value.routing.plan.revision, overrides: overrides.value, choices: choices.value } : null, () => submitting.value)
  const compatibility = computed(() => autoPreview.result.value?.compatibility ?? preview.value?.compatibility ?? view.value?.compatibility ?? [])
  const errors = computed(() => ({ ...autoPreview.result.value?.fieldErrors, ...fieldErrors.value }))
  async function load(preserve = true) {
    controller?.abort(); controller = new globalThis.AbortController()
    const ticket = latest.begin(); loading.value = true; error.value = null
    try {
      const value = await xnodeMemberApi.diy(controller.signal)
      if (!latest.isCurrent(ticket)) return
      view.value = value
      if (!hydrated || !preserve) { overrides.value = globalThis.structuredClone(value.overrides); choices.value = globalThis.structuredClone(value.routing.plan.choices); hydrated = true }
    } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'diy.loadFailed') }
    finally { if (latest.isCurrent(ticket)) loading.value = false }
  }
  function routeDraft(value: RoutingChoice[]) { if (JSON.stringify(choices.value) !== JSON.stringify(value)) saved.value = false; choices.value = value }
  function reset() { overrides.value = {}; fieldErrors.value = {}; preview.value = null; saved.value = false }
  async function save() {
    if (!view.value || submitting.value) return
    const parsed = diySchema(view.value.fields).safeParse(overrides.value)
    fieldErrors.value = {}; error.value = null; saved.value = false
    if (!parsed.success) {
      const codes = new Set(['range', 'conflict', 'dependency', 'serverCapability'])
      for (const issue of parsed.error.issues) fieldErrors.value[String(issue.path[0] ?? 'overrides')] = codes.has(issue.message) ? issue.message : 'invalid'
      error.value = t('diy.validationFailed'); return
    }
    const body: DIYWrite = JSON.parse(JSON.stringify({ settingsRevision: view.value.settingsRevision, routingRevision: view.value.routing.plan.revision, overrides: parsed.data, choices: choices.value })) as DIYWrite
    const fingerprint = JSON.stringify(body), key = keys.get(fingerprint) ?? createUuid()
    keys.set(fingerprint, key); submitting.value = true
    validator?.abort(); validator = new globalThis.AbortController()
    try {
      const validation = await xnodeMemberApi.previewDIY(body, validator.signal)
      if (disposed) return
      preview.value = validation; fieldErrors.value = validation.fieldErrors
      if (!validation.valid) { error.value = t('diy.validationFailed'); return }
      const result = await xnodeMemberApi.saveDIY(body, key)
      if (disposed) return
      keys.delete(fingerprint); saved.value = true
      if (view.value) view.value = { ...view.value, settingsRevision: result.settingsRevision, overrides: body.overrides, routing: { ...view.value.routing, plan: { ...view.value.routing.plan, choices: body.choices, ...(result.routingChanged ? { revision: body.routingRevision + 1, phase: 'revoking', operationId: result.receipt.id } : {}) } } }
      if (result.routingChanged) tracker.track(result.receipt)
      void load(true)
    } catch (caught) {
      if (!disposed) error.value = caught && typeof caught === 'object' && 'code' in caught && caught.code === 'XNODE_CONFLICT' ? t('diy.conflict') : localizedError(caught, 'diy.saveFailed')
    } finally { if (!disposed) submitting.value = false }
  }
  onMounted(() => void load(false))
  watch(overrides, () => { saved.value = false; preview.value = null; fieldErrors.value = {} }, { deep: true })
  onScopeDispose(() => { disposed = true; latest.dispose(); controller?.abort(); validator?.abort(); keys.clear() })
  return { view, overrides, choices, loading, submitting, saved, error, errors, compatibility, tracker, autoPreview, load, save, reset, routeDraft }
}
