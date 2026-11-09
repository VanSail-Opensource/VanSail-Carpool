import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeApi } from '@/api/xnode'
import { ApiError } from '@/api/http'
import { adminOperationsApi } from '@/api/adminOperations'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { localizedError } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { createCertificateAttempt, loadCertificateAttempt, saveCertificateAttempt, type CertificateAction, type CertificateAttempt } from './certificateAttempts'

interface CertificateLifecycleOptions {
  nodeId: () => string
  profileId: () => string
  revision: () => number
  canRun: () => boolean
}
export function useCertificateLifecycle(options: CertificateLifecycleOptions) {
  const session = useSessionStore()
  const scope = () => `${session.user?.id ?? ''}:${options.nodeId()}:${options.profileId()}`
  const attempt = shallowRef<CertificateAttempt | null>(null)
  const recovering = shallowRef(false), recovered = shallowRef(false), retired = shallowRef(false)
  const recoveryError = shallowRef<string | null>(null)
  const lastAction = shallowRef<CertificateAction | null>(null)
  let generation = 0, controller: AbortController | undefined
  function clearAttempt(expectedScope = scope()): void { saveCertificateAttempt(expectedScope, null); if (scope() === expectedScope) attempt.value = null }
  const command = useDurableCommand({ errorKey: 'certificates.actionFailed', loadReceipt: adminOperationsApi.getOperation, onTerminal: receipt => {
    if (['succeeded', 'failed', 'cancelled'].includes(receipt.status)) clearAttempt()
  } })
  const uncertain = computed(() => !!attempt.value && !command.receipt.value)
  const blocksMutations = computed(() => retired.value || !recovered.value || recovering.value || !!recoveryError.value || command.busy.value || command.blocksMutations.value || uncertain.value)
  const canRetry = computed(() => command.receipt.value?.status === 'failed' && !!lastAction.value)
  const error = computed(() => recoveryError.value ?? command.error.value)

  async function recover(): Promise<void> {
    if (!options.nodeId() || retired.value) return
    controller?.abort(); controller = new AbortController()
    const ticket = ++generation
    recovering.value = true; recoveryError.value = null
    try {
      // A local unresolved request takes precedence over older profile history.
      if (attempt.value) {
        const accepted = await submit(attempt.value)
        if (ticket !== generation || retired.value) return
        if (accepted || attempt.value) { recovered.value = true; return }
      }
      const value = await xnodeApi.certificateOperation(options.nodeId(), options.profileId(), controller.signal)
      if (ticket !== generation || retired.value) return
      lastAction.value = value.result.action === 'issue' || value.result.action === 'renew' ? value.result.action : null
      if (value.receipt) {
        command.resume(value.receipt, `${options.nodeId()}:${options.profileId()}`)
        if (['succeeded', 'failed', 'cancelled'].includes(value.receipt.status)) clearAttempt()
      }
      recovered.value = true
    } catch (caught) { if (ticket === generation) recoveryError.value = localizedError(caught, 'certificates.recoveryFailed') }
    finally { if (ticket === generation) recovering.value = false }
  }
  async function submit(original: CertificateAttempt): Promise<boolean> {
    const originalScope = scope()
    attempt.value = original; saveCertificateAttempt(scope(), original); lastAction.value = original.action
    const node = options.nodeId(), profile = options.profileId()
    const accepted = await command.execute(`${node}:${profile}`, `${node}:${profile}:${original.key}`, async () => {
      try {
        return await xnodeApi.command(node, { name: 'certificates.run', profileId: profile, action: original.action, expectedRevision: original.expectedRevision }, original.key)
      } catch (caught) {
        if (caught instanceof ApiError && [400, 401, 403, 404, 409, 422].includes(caught.status)) clearAttempt(originalScope)
        throw caught
      }
    })
    if (accepted) clearAttempt()
    return accepted
  }
  async function run(action: CertificateAction): Promise<void> {
    if (blocksMutations.value || !options.canRun() || !Number.isSafeInteger(options.revision()) || options.revision() < 1) return
    await submit(createCertificateAttempt(action, options.revision()))
  }
  async function reconcile(): Promise<void> {
    await recover()
    if (!retired.value && command.receipt.value) await command.refresh()
  }
  function retire(): void { retired.value = true; generation++; controller?.abort(); command.reset(); attempt.value = null }
  watch([options.nodeId, options.profileId], () => {
    generation++; controller?.abort(); command.reset(); recovered.value = false; recoveryError.value = null
    attempt.value = loadCertificateAttempt(scope()); lastAction.value = attempt.value?.action ?? null
    if (options.nodeId()) void recover()
    else recovered.value = true
  }, { immediate: true })
  window.addEventListener('txc:session-cleared', retire)
  window.addEventListener('txc:browser-session-expired', retire)
  onScopeDispose(() => { retire(); window.removeEventListener('txc:session-cleared', retire); window.removeEventListener('txc:browser-session-expired', retire) })
  return { receipt: command.receipt, checking: command.checking, busy: command.busy, error, recovering, blocksMutations, uncertain, canRetry, lastAction, run, refresh: reconcile }
}
