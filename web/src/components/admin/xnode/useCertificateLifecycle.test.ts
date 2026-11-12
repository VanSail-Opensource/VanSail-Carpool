import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/http'
import type { OperationReceipt } from '@/api/types'
import { loadCertificateAttempt } from './certificateAttempts'

const api = vi.hoisted(() => ({ certificateOperation: vi.fn(), command: vi.fn(), getOperation: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
vi.mock('@/api/adminOperations', () => ({ adminOperationsApi: { getOperation: api.getOperation } }))
vi.mock('@/stores/session', () => ({ useSessionStore: () => ({ user: { id: 'administrator' } }) }))
vi.mock('@/i18n', () => ({ localizedError: (_caught: unknown, key: string) => key }))
import { useCertificateLifecycle } from './useCertificateLifecycle'

const receipt = (status: OperationReceipt['status'], id = 'original-operation'): OperationReceipt => ({ id, kind: 'xnode_command', status, createdAt: '2026-11-09T00:00:00Z', updatedAt: '2026-11-09T00:00:00Z' })
function setup(revision = 3, canRun = true) {
  let state!: ReturnType<typeof useCertificateLifecycle>
  const wrapper = mount(defineComponent({ setup() {
    state = useCertificateLifecycle({ nodeId: () => 'node', profileId: () => 'primary', revision: () => revision, canRun: () => canRun })
    return () => null
  } }))
  return { wrapper, state }
}

beforeEach(() => {
  vi.useFakeTimers(); vi.resetAllMocks(); sessionStorage.clear()
  api.certificateOperation.mockResolvedValue({ receipt: null, result: {} })
  api.command.mockResolvedValue(receipt('queued'))
  api.getOperation.mockResolvedValue(receipt('succeeded'))
})
afterEach(() => { vi.useRealTimers() })

describe('durable certificate operations', () => {
  it('submits revision-checked actions and resumes their original receipts', async () => {
    const { wrapper, state } = setup()
    await flushPromises(); await state.run('issue')
    expect(api.command.mock.calls[0]?.slice(0, 2)).toEqual(['node', { name: 'certificates.run', profileId: 'primary', action: 'issue', expectedRevision: 3 }])
    expect(state.blocksMutations.value).toBe(true)
    await vi.advanceTimersByTimeAsync(1500)
    expect(api.getOperation).toHaveBeenCalledWith('original-operation')
    expect(state.receipt.value?.status).toBe('succeeded')
    wrapper.unmount()
  })

  it('replays an unresolved original key before reading potentially older terminal history', async () => {
    api.command.mockRejectedValueOnce(new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }))
    const first = setup()
    await flushPromises(); await first.state.run('renew')
    const key = api.command.mock.calls[0]?.[2]
    expect(loadCertificateAttempt('administrator:node:primary')).toMatchObject({ key, action: 'renew', expectedRevision: 3 })
    expect(first.state.uncertain.value).toBe(true)
    first.wrapper.unmount()
    api.certificateOperation.mockResolvedValue({ receipt: receipt('failed', 'older-failed-operation'), result: { action: 'issue', expected_revision: 1 } })
    const second = setup(9)
    await flushPromises()
    expect(api.command.mock.calls[1]).toEqual(['node', { name: 'certificates.run', profileId: 'primary', action: 'renew', expectedRevision: 3 }, key])
    expect(api.certificateOperation).toHaveBeenCalledTimes(1)
    expect(second.state.receipt.value?.id).toBe('original-operation')
    expect(second.state.lastAction.value).toBe('renew')
    second.wrapper.unmount()
  })

  it('keeps uncertain receipts blocked and allows only status reconciliation', async () => {
    api.certificateOperation.mockResolvedValue({ receipt: receipt('pending_review'), result: { action: 'renew', expected_revision: 3 } })
    api.getOperation.mockResolvedValue(receipt('pending_review'))
    const { wrapper, state } = setup()
    await flushPromises(); await state.run('issue'); await state.refresh()
    expect(state.blocksMutations.value).toBe(true)
    expect(state.canRetry.value).toBe(false)
    expect(api.command).not.toHaveBeenCalled()
    expect(api.getOperation).toHaveBeenCalledWith('original-operation')
    wrapper.unmount()
  })

  it('retries only a definitive failure and respects capability/draft eligibility', async () => {
    api.certificateOperation.mockResolvedValue({ receipt: receipt('failed'), result: { action: 'renew', expected_revision: 3 } })
    const allowed = setup()
    await flushPromises()
    expect(allowed.state.canRetry.value).toBe(true)
    await allowed.state.run(allowed.state.lastAction.value!)
    expect(api.command.mock.calls[0]?.[1]).toMatchObject({ action: 'renew' })
    allowed.wrapper.unmount()
    api.command.mockClear()
    const blocked = setup(3, false)
    await flushPromises(); await blocked.state.run('issue')
    expect(api.command).not.toHaveBeenCalled()
    blocked.wrapper.unmount()
  })

  it('retains unknown attempts on transport failure and clears them only on definite rejection or session end', async () => {
    api.command.mockRejectedValueOnce(new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }))
    const { wrapper, state } = setup()
    await flushPromises(); await state.run('issue')
    expect(state.uncertain.value).toBe(true)
    api.command.mockRejectedValueOnce(new ApiError(422, { code: 'XNODE_INPUT_INVALID', message: '' }))
    await state.refresh()
    expect(loadCertificateAttempt('administrator:node:primary')).toBeNull()
    window.dispatchEvent(new Event('txc:browser-session-expired'))
    expect(state.blocksMutations.value).toBe(true)
    wrapper.unmount()
  })

  it('recovers history after verified reauthentication but remains retired after logout', async () => {
    const { wrapper, state } = setup()
    await flushPromises()
    expect(state.blocksMutations.value).toBe(false)
    window.dispatchEvent(new Event('txc:browser-session-expired'))
    expect(state.blocksMutations.value).toBe(true)
    window.dispatchEvent(new Event('txc:browser-session-restored'))
    await flushPromises()
    expect(api.certificateOperation).toHaveBeenCalledTimes(2)
    expect(state.blocksMutations.value).toBe(false)
    window.dispatchEvent(new Event('txc:session-cleared'))
    window.dispatchEvent(new Event('txc:browser-session-restored'))
    await flushPromises()
    expect(state.blocksMutations.value).toBe(true)
    expect(api.certificateOperation).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
})
