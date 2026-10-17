import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { memberOperationsApi } from '@/api/memberOperations'
import type { OperationReceipt } from '@/api/types'
import { operationReceiptLoaderKey } from './operationReceiptLoader'
import { useOperationReceipt } from './useOperationReceipt'

vi.mock('@/api/memberOperations', () => ({ memberOperationsApi: { getOperation: vi.fn() } }))
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks() })

describe('experience receipt loader', () => {
  it('polls the provided admin reader and cancels its timer on disposal', async () => {
    vi.useFakeTimers()
    const receipt: OperationReceipt = { id: 'browser-command', kind: 'xnode_command', status: 'queued', createdAt: '2026-10-18T00:00:00Z', updatedAt: '2026-10-18T00:00:00Z' }
    const read = vi.fn().mockResolvedValue({ ...receipt, status: 'processing' })
    let tracker!: ReturnType<typeof useOperationReceipt>
    const wrapper = mount(defineComponent({ setup() { tracker = useOperationReceipt(20); return () => h('div') } }), { global: { provide: { [operationReceiptLoaderKey as symbol]: read } } })
    tracker.track(receipt)
    await vi.advanceTimersByTimeAsync(20)
    expect(read).toHaveBeenCalledWith(receipt.id)
    expect(memberOperationsApi.getOperation).not.toHaveBeenCalled()
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(1000)
    expect(read).toHaveBeenCalledTimes(1)
  })
})
