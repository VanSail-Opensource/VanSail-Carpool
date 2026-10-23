import { defineComponent, shallowRef } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import { xnodeMemberApi, type RoutingView } from '@/api/xnodeMember'
import { memberOperationsApi } from '@/api/memberOperations'
import { useMemberRouting } from './useMemberRouting'

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

function mountRouting(options: Parameters<typeof useMemberRouting>[0]) {
  let state!: ReturnType<typeof useMemberRouting>
  const wrapper = mount(defineComponent({ setup() { state = useMemberRouting(options); return () => null } }))
  return { state, wrapper }
}

it('resumes an existing deployment, refreshes its phase and retains a local draft', async () => {
  vi.useFakeTimers()
  const initial = shallowRef<RoutingView>({ plan: { revision: 1, phase: 'revoking', operationId: 'existing', choices: [] }, nodes: [], endpoints: [], templates: [], paths: [] })
  const receipt = { id: 'existing', kind: 'xnode_diy', status: 'processing' as const, createdAt: '', updatedAt: '' }
  const getOperation = vi.spyOn(memberOperationsApi, 'getOperation').mockResolvedValueOnce(receipt).mockResolvedValue({ ...receipt, status: 'succeeded' })
  vi.spyOn(xnodeMemberApi, 'previewRouting').mockResolvedValue({ paths: [], trace: null })
  const onRefresh = vi.fn(() => { initial.value = { ...initial.value, plan: { ...initial.value.plan, phase: 'active' } } })
  const { state, wrapper } = mountRouting({ initial: () => initial.value, onDraft: vi.fn(), onRefresh })
  try {
    await flushPromises()
    expect(state.locked.value).toBe(true)
    state.choices.value = [{ id: 'draft', inboundNodeId: 'a', outboundNodeId: 'b', templateId: 'one' }]
    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()
    expect(getOperation).toHaveBeenCalledWith('existing')
    expect(onRefresh).toHaveBeenCalledOnce()
    expect(state.current.value?.plan.phase).toBe('active')
    expect(state.locked.value).toBe(false)
    expect(state.choices.value[0]?.id).toBe('draft')
  } finally { wrapper.unmount() }
})

it('retains a recovered review receipt without polling or unlocking its unfinished plan', async () => {
  vi.useFakeTimers()
  const initial: RoutingView = { plan: { revision: 1, phase: 'revoking', operationId: 'review', choices: [] }, nodes: [], endpoints: [], templates: [], paths: [] }
  const getOperation = vi.spyOn(memberOperationsApi, 'getOperation').mockResolvedValue({ id: 'review', kind: 'xnode_routing', status: 'pending_review', createdAt: '', updatedAt: '' })
  vi.spyOn(xnodeMemberApi, 'previewRouting').mockResolvedValue({ paths: [], trace: null })
  const onRefresh = vi.fn()
  const { state, wrapper } = mountRouting({ initial: () => initial, onDraft: vi.fn(), onRefresh })
  try {
    await flushPromises()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(state.command.receipt.value?.status).toBe('pending_review')
    expect(state.locked.value).toBe(true)
    expect(getOperation).toHaveBeenCalledOnce()
    expect(onRefresh).toHaveBeenCalledOnce()
  } finally { wrapper.unmount() }
})
