import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { OperationReceipt } from '@/api/types'
import { xnodeApi } from '@/api/xnode'
import XNodeCommands from './XNodeCommands.vue'
import XNodeMetricDetails from './XNodeMetricDetails.vue'

const command = vi.hoisted(() => ({ execute: vi.fn().mockResolvedValue(true), onTerminal: null as null | ((receipt: OperationReceipt) => Promise<void>) }))
vi.mock('@/composables/useDurableCommand', async () => {
  const { shallowRef } = await import('vue')
  return { useDurableCommand: (options: { onTerminal: (receipt: OperationReceipt) => Promise<void> }) => {
    command.onTerminal = options.onTerminal
    return { execute: command.execute, receipt: shallowRef(null), error: shallowRef(null), checking: shallowRef(false), busy: shallowRef(false), blocksMutations: shallowRef(false), refresh: vi.fn() }
  } }
})

describe('node command result ownership', () => {
  it('rejects an old result once another command starts', async () => {
    let finish!: (value: Awaited<ReturnType<typeof xnodeApi.commandResult>>) => void
    const result = vi.spyOn(xnodeApi, 'commandResult').mockReturnValue(new Promise(resolve => { finish = resolve }))
    const wrapper = shallowMount(XNodeCommands, { props: { nodeId: 'node', endpoints: [] }, global: { renderStubDefaultSlot: true } })
    const receipt = { id: 'old-command', kind: 'xnode_command', status: 'succeeded', createdAt: '', updatedAt: '' } satisfies OperationReceipt
    const pending = command.onTerminal!(receipt)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    finish({ receipt, result: { previous: true } })
    await pending
    expect(wrapper.findComponent(XNodeMetricDetails).exists()).toBe(false)
    expect(command.execute).toHaveBeenCalledWith('node:runtime.reload', JSON.stringify(['node', { name: 'runtime.reload' }]), expect.any(Function))
    wrapper.unmount()
    result.mockRestore()
  })
})
