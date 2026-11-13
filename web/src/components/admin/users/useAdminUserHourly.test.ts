import { effectScope, shallowRef } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { adminOperationsApi, type AdminHourlySquad, type AdminHourlyView, type OperationReceipt } from '@/api/adminOperations'
import { ApiError } from '@/api/http'
import { useAdminUserHourly } from './useAdminUserHourly'

afterEach(() => vi.restoreAllMocks())
const squad = { enrollmentId: 'enrollment' } as AdminHourlySquad
const snapshot = (version: string) => ({ policy: { version }, squads: [squad] }) as AdminHourlyView

describe('administrator hourly recovery and ownership', () => {
  it('reuses an uncertain key only for the original owner, enrollment and policy', async () => {
    const get = vi.spyOn(adminOperationsApi, 'getAdminHourly').mockResolvedValue(snapshot('original-policy'))
    const disable = vi.spyOn(adminOperationsApi, 'disableAdminHourly').mockRejectedValue(new ApiError(503, { code: 'TEMPORARY_FAILURE', message: '' }))
    const scope = effectScope()
    const state = scope.run(() => useAdminUserHourly('member'))!
    await flushPromises()
    await state.disable(squad); await state.disable(squad)
    expect(disable.mock.calls[1]![3]).toBe(disable.mock.calls[0]![3])
    get.mockResolvedValue(snapshot('new-policy'))
    await state.load()
    await state.disable(squad)
    expect(disable.mock.calls[2]![3]).not.toBe(disable.mock.calls[0]![3])
    expect(disable.mock.calls[2]!.slice(0, 3)).toEqual(['member', 'enrollment', 'new-policy'])
    scope.stop()
  })

  it('does not apply the previous member receipt or clear the new member pending action', async () => {
    vi.spyOn(adminOperationsApi, 'getAdminHourly').mockResolvedValue(snapshot('policy'))
    let finishFirst!: (receipt: OperationReceipt) => void
    let finishSecond!: (receipt: OperationReceipt) => void
    vi.spyOn(adminOperationsApi, 'disableAdminHourly')
      .mockReturnValueOnce(new Promise(resolve => { finishFirst = resolve }))
      .mockReturnValueOnce(new Promise(resolve => { finishSecond = resolve }))
    const owner = shallowRef('first-member')
    const scope = effectScope()
    const state = scope.run(() => useAdminUserHourly(owner))!
    await flushPromises()
    const first = state.disable(squad)
    owner.value = 'second-member'
    await flushPromises()
    const second = state.disable(squad)
    const receipt = { id: 'first', kind: 'hourly_access', status: 'queued', createdAt: '', updatedAt: '', completedAt: null } satisfies OperationReceipt
    finishFirst(receipt)
    expect(await first).toBe(false)
    expect(state.queued.value).toBeNull()
    expect(state.busyEnrollment.value).toBe('enrollment')
    finishSecond({ ...receipt, id: 'second' })
    expect(await second).toBe(true)
    expect(state.queued.value?.id).toBe('second')
    scope.stop()
  })
})
