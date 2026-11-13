import { defineComponent, shallowRef } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { adminOperationsApi, type AdminUserDetail } from '@/api/adminOperations'
import { ApiError } from '@/api/http'
import { clearResponseCache } from '@/api/cache/session'
import { useAdminUserProfile } from './useAdminUserProfile'

afterEach(() => { vi.restoreAllMocks(); clearResponseCache() })

function mountProfile() {
  let state!: ReturnType<typeof useAdminUserProfile>
  const wrapper = mount(defineComponent({ setup() { state = useAdminUserProfile(shallowRef('member')); return () => null } }))
  return { state, wrapper }
}

describe('administrator profile mutation recovery', () => {
  it('retains ambiguous HTTP attempts for their exact payload and releases accepted attempts', async () => {
    vi.spyOn(adminOperationsApi, 'getUser').mockResolvedValue({ entitlements: [], activeCombo: null } as unknown as AdminUserDetail)
    const { state, wrapper } = mountProfile()
    await flushPromises()
    const task = vi.fn().mockRejectedValue(new ApiError(503, { code: 'TEMPORARY_FAILURE', message: '' }))
    const first = { amountTxbMinor: '100', reason: 'Support correction' }
    await state.perform('refund:purchase', task, { intent: first })
    await state.perform('refund:purchase', task, { intent: first })
    await state.perform('refund:purchase', task, { intent: { ...first, amountTxbMinor: '200' } })
    expect(task.mock.calls[1]![0]).toBe(task.mock.calls[0]![0])
    expect(task.mock.calls[2]![0]).not.toBe(task.mock.calls[0]![0])
    task.mockResolvedValue({})
    await state.perform('refund:purchase', task, { intent: first })
    expect(task.mock.calls[3]![0]).toBe(task.mock.calls[0]![0])
    await state.perform('refund:purchase', task, { intent: first })
    expect(task.mock.calls[4]![0]).not.toBe(task.mock.calls[0]![0])
    wrapper.unmount()
  })

  it('keeps the current profile mounted through a failed quiet refresh and recovers', async () => {
    const original = { entitlements: [], activeCombo: null } as unknown as AdminUserDetail
    const read = vi.spyOn(adminOperationsApi, 'getUser').mockResolvedValue(original)
    const { state, wrapper } = mountProfile()
    await flushPromises()
    let reject!: (reason: Error) => void
    read.mockReturnValueOnce(new Promise((_resolve, fail) => { reject = fail }))
    const pending = state.load(true)
    expect(state.loading.value).toBe(false)
    expect(state.detail.value).toMatchObject(original)
    reject(new Error('profile read unavailable'))
    await pending
    expect(state.loading.value).toBe(false)
    expect(state.detail.value).toMatchObject(original)
    expect(state.error.value).toBeTruthy()
    const mutate = vi.fn()
    expect(await state.perform('refund:purchase', mutate)).toBe(false)
    expect(mutate).not.toHaveBeenCalled()
    await state.load(true)
    expect(state.error.value).toBeNull()
    expect(state.detail.value).toMatchObject(original)
    wrapper.unmount()
  })
})
