import { computed, onMounted, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { xnodeMemberApi, type DIYView, type DIYSplitProfile, type DIYOverrides, type DIYWrite, type DIYPreview, type RoutingChoice } from '@/api/xnodeMember'
import { ApiError } from '@/api/http'
import { restoreCached } from '@/api/cache/restore'
import { notifyHaptic } from '@/utils/telegramHaptics'
import { memberOperationsApi } from '@/api/memberOperations'
import { useOperationReceipt } from '@/composables/useOperationReceipt'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { createLatestRequest } from '@/utils/latestRequest'
import { activeDIYOverrides } from './diyApplicability'
import { diySchema } from './diyValidation'
import { useDIYPreview } from './useDIYPreview'

export function useDIY() {
  const view = shallowRef<DIYView | null>(null)
  const splitProfiles = ref<DIYSplitProfile[]>([])
  const overrides = ref<DIYOverrides>({}), choices = ref<RoutingChoice[]>([])
  const preview = shallowRef<DIYPreview | null>(null)
  const fieldErrors = shallowRef<Record<string, string>>({})
  const loading = shallowRef(true), submitting = shallowRef(false), saved = shallowRef(false)
  const confirmed = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const latest = createLatestRequest(), keys = new Map<string, { key: string; body: DIYWrite }>()
  let controller: InstanceType<typeof globalThis.AbortController> | undefined
  let validator: InstanceType<typeof globalThis.AbortController> | undefined
  let disposed = false, hydrated = false
  let restoredDrafts: { overrides: string; splits: string; choices: string } | undefined
  let inventoryTimer: ReturnType<typeof globalThis.setTimeout> | undefined
  const tracker = useOperationReceipt(1500, async id => {
    const receipt = await memberOperationsApi.getOperation(id)
    if (!disposed && !['queued', 'processing'].includes(receipt.status)) void load(true)
    return receipt
  })
  const activeOverrides = computed(() => view.value ? activeDIYOverrides(view.value.fields, overrides.value) : {})
  const invalid = computed(() => view.value ? !diySchema(view.value.fields).safeParse(activeOverrides.value).success : false)
  const autoPreview = useDIYPreview(() => view.value && !invalid.value ? { settingsRevision: view.value.settingsRevision, routingRevision: view.value.routing.plan.revision, overrides: activeOverrides.value, splitProfiles: splitProfiles.value, choices: choices.value } : null, () => submitting.value || loading.value || !confirmed.value)
  const compatibility = computed(() => autoPreview.result.value?.compatibility ?? preview.value?.compatibility ?? view.value?.compatibility ?? [])
  const errors = computed(() => ({ ...autoPreview.result.value?.fieldErrors, ...fieldErrors.value }))
  function scheduleInventoryRefresh() {
    inventoryTimer = globalThis.setTimeout(() => {
      if (disposed) return
      if (submitting.value) scheduleInventoryRefresh()
      else void load(true)
    }, 1500)
  }
  async function load(preserve = true) {
    if (disposed) return
    if (inventoryTimer) globalThis.clearTimeout(inventoryTimer)
    controller?.abort(); controller = new globalThis.AbortController()
    const ticket = latest.begin(); loading.value = true; confirmed.value = false; error.value = null
    try {
      const value = await xnodeMemberApi.diy(controller.signal)
      if (!latest.isCurrent(ticket)) return
      view.value = value
      if (!hydrated || !preserve) { overrides.value = globalThis.structuredClone(value.overrides); splitProfiles.value = globalThis.structuredClone(value.splitProfiles ?? []); choices.value = globalThis.structuredClone(value.routing.plan.choices); hydrated = true }
      else if (restoredDrafts) {
        // An instant snapshot may already be editable while this live read runs.
        // Refresh untouched sections without replacing edits made in that gap.
        if (JSON.stringify(overrides.value) === restoredDrafts.overrides) overrides.value = globalThis.structuredClone(value.overrides)
        if (JSON.stringify(splitProfiles.value) === restoredDrafts.splits) splitProfiles.value = globalThis.structuredClone(value.splitProfiles ?? [])
        if (JSON.stringify(choices.value) === restoredDrafts.choices) choices.value = globalThis.structuredClone(value.routing.plan.choices)
      }
      restoredDrafts = undefined; confirmed.value = true
      if (value.inventoryPending) scheduleInventoryRefresh()
    } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'diy.loadFailed') }
    finally { if (latest.isCurrent(ticket)) loading.value = false }
  }
  function routeDraft(value: RoutingChoice[]) { if (JSON.stringify(choices.value) !== JSON.stringify(value)) saved.value = false; choices.value = value }
  function reset() { overrides.value = {}; fieldErrors.value = {}; preview.value = null; saved.value = false }
  async function save() {
    if (!view.value || submitting.value || loading.value || !confirmed.value) return
    const parsed = diySchema(view.value.fields).safeParse(activeDIYOverrides(view.value.fields, overrides.value))
    fieldErrors.value = {}; error.value = null; saved.value = false
    if (!parsed.success) {
      const codes = new Set(['range', 'conflict', 'dependency', 'serverCapability'])
      for (const issue of parsed.error.issues) fieldErrors.value[String(issue.path[0] ?? 'overrides')] = codes.has(issue.message) ? issue.message : 'invalid'
      error.value = t('diy.validationFailed'); notifyHaptic('error'); return
    }
    const draft: DIYWrite = JSON.parse(JSON.stringify({ settingsRevision: view.value.settingsRevision, routingRevision: view.value.routing.plan.revision, overrides: parsed.data, splitProfiles: splitProfiles.value, choices: choices.value })) as DIYWrite
    const fingerprint = JSON.stringify(draft), retry = keys.get(fingerprint), key = retry?.key ?? createUuid()
    const body = retry?.body ?? draft
    submitting.value = true
    validator?.abort(); validator = new globalThis.AbortController()
    try {
      if (!retry) {
        const validation = await xnodeMemberApi.previewDIY(body, validator.signal)
        if (disposed) return
        preview.value = validation; fieldErrors.value = validation.fieldErrors
        if (!validation.valid) { error.value = t('diy.validationFailed'); notifyHaptic('error'); return }
      }
      keys.set(fingerprint, { key, body })
      const result = await xnodeMemberApi.saveDIY(body, key)
      if (disposed) return
      keys.delete(fingerprint); saved.value = true; notifyHaptic('success')
      if (view.value) view.value = { ...view.value, settingsRevision: result.settingsRevision, overrides: body.overrides, routing: { ...view.value.routing, plan: { ...view.value.routing.plan, choices: body.choices, ...(result.routingChanged ? { revision: body.routingRevision + 1, phase: 'revoking', operationId: result.receipt.id } : {}) } } }
      if (result.deploymentChanged || result.routingChanged) tracker.track(result.receipt)
      void load(true)
    } catch (caught) {
      if (!disposed) notifyHaptic('error')
      if (!disposed) error.value = caught && typeof caught === 'object' && 'code' in caught && caught.code === 'XNODE_CONFLICT' ? t('diy.conflict') : localizedError(caught, 'diy.saveFailed')
      if (caught instanceof ApiError && caught.status >= 400 && caught.status < 500) keys.delete(fingerprint)
    } finally { if (!disposed) submitting.value = false }
  }
  restoreCached<DIYView>('/api/v1/xnode/diy', value => {
    view.value = value
    overrides.value = globalThis.structuredClone(value.overrides)
    splitProfiles.value = globalThis.structuredClone(value.splitProfiles ?? [])
    choices.value = globalThis.structuredClone(value.routing.plan.choices)
    restoredDrafts = { overrides: JSON.stringify(overrides.value), splits: JSON.stringify(splitProfiles.value), choices: JSON.stringify(choices.value) }
    hydrated = true
  }, {}, 60_000)
  onMounted(() => void load())
  watch([overrides, splitProfiles], () => { saved.value = false; preview.value = null; fieldErrors.value = {} }, { deep: true })
  onScopeDispose(() => { disposed = true; latest.dispose(); controller?.abort(); validator?.abort(); if (inventoryTimer) globalThis.clearTimeout(inventoryTimer); keys.clear() })
  return { invalid, view, overrides, splitProfiles, choices, loading, confirmed, submitting, saved, error, errors, compatibility, tracker, autoPreview, load, save, reset, routeDraft }
}
