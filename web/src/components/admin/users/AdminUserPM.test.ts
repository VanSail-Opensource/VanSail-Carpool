import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { pmApi } from '@/api/pm'
import type { OperationReceipt, PMConversation } from '@/api/types'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import { operationReceiptLoaderKey } from '@/composables/operationReceiptLoader'
import PMTopicRepairDialog from '../pm/PMTopicRepairDialog.vue'
import AdminUserPM from './AdminUserPM.vue'

const conversation: PMConversation = {
  id: 'conversation', userId: 'member', telegramId: '42', name: 'Member', username: '',
  chatId: '-10042', topicId: '12', profileMessageId: '13', topicState: 'ready', profileState: 'ready',
  blocked: false, muted: false, createdAt: '2026-11-13T00:00:00Z', updatedAt: '2026-11-13T00:00:00Z',
}
const queued: OperationReceipt = {
  id: 'moderation', kind: 'telegram_pm_profile', status: 'queued',
  createdAt: conversation.createdAt, updatedAt: conversation.updatedAt,
}

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

describe('PM moderation receipt lifetime', () => {
  it('retains an ambiguous error and replay key instead of requesting a parent reload', async () => {
    const moderate = vi.spyOn(pmApi, 'moderate').mockRejectedValue(new Error('response lost after commit'))
    const wrapper = shallowMount(AdminUserPM, { props: { conversation } })
    const control = wrapper.findAllComponents(SwitchField)[0]!
    control.vm.$emit('update:modelValue', true)
    await flushPromises()
    expect(wrapper.emitted('changed')).toBeUndefined()
    expect(wrapper.getComponent(OperationStatusNotice).props('error')).toBeTruthy()
    control.vm.$emit('update:modelValue', true)
    await flushPromises()
    expect(moderate).toHaveBeenCalledTimes(2)
    expect(moderate.mock.calls[0]).toEqual([conversation.id, { blocked: true }, expect.any(String)])
    expect(moderate.mock.calls[1][2]).toBe(moderate.mock.calls[0][2])
    expect(wrapper.emitted('changed')).toBeUndefined()
    wrapper.unmount()
  })

  it('refreshes accepted flags while retaining the queued receipt through terminal polling', async () => {
    vi.useFakeTimers()
    const moderate = vi.spyOn(pmApi, 'moderate').mockResolvedValue(queued)
    const read = vi.fn().mockResolvedValue({ ...queued, status: 'succeeded' })
    const wrapper = shallowMount(AdminUserPM, {
      props: { conversation }, global: { provide: { [operationReceiptLoaderKey as symbol]: read } },
    })
    const control = wrapper.findAllComponents(SwitchField)[0]!
    control.vm.$emit('update:modelValue', true)
    await flushPromises()
    expect(wrapper.emitted('changed')).toHaveLength(1)
    expect(control.props('disabled')).toBe(true)
    expect(wrapper.getComponent(OperationStatusNotice).props('receipt')).toEqual(queued)
    control.vm.$emit('update:modelValue', false)
    await flushPromises()
    expect(moderate).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()
    expect(read).toHaveBeenCalledExactlyOnceWith(queued.id)
    expect(wrapper.emitted('changed')).toHaveLength(2)
    expect(control.props('disabled')).toBe(false)
    wrapper.unmount()
  })

  it('permits an explicit topic repair after an accepted pending-review receipt', async () => {
    vi.useFakeTimers()
    vi.spyOn(pmApi, 'moderate').mockResolvedValue({ ...queued, status: 'pending_review' })
    const repair = vi.spyOn(pmApi, 'repair').mockResolvedValue({ ...queued, id: 'repair', kind: 'telegram_pm_repair' })
    const wrapper = shallowMount(AdminUserPM, { props: { conversation: { ...conversation, profileState: 'pending_review' } } })
    wrapper.findAllComponents(SwitchField)[0]!.vm.$emit('update:modelValue', true)
    await flushPromises()
    expect(wrapper.emitted('changed')).toHaveLength(1)
    const body = { topicId: '12', profileMessageId: '13' }
    wrapper.getComponent(PMTopicRepairDialog).vm.$emit('confirm', body)
    await flushPromises()
    expect(repair).toHaveBeenCalledExactlyOnceWith(conversation.id, body, expect.any(String))
    expect(wrapper.getComponent(OperationStatusNotice).props('receipt')).toMatchObject({ id: 'repair', status: 'queued' })
    expect(wrapper.emitted('changed')).toHaveLength(1)
    wrapper.unmount()
  })
})
