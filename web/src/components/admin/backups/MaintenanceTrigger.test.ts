/* eslint-disable vue/one-component-per-file */

import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  queue: vi.fn(),
  getOperation: vi.fn(),
}))

vi.mock('@/api/client', () => ({ api: { runAdminMaintenance: mocks.queue } }))
vi.mock('@/api/memberOperations', () => ({ memberOperationsApi: { getOperation: mocks.getOperation } }))

import MaintenanceTrigger from './MaintenanceTrigger.vue'

const ButtonStub = defineComponent({
  inheritAttrs: false,
  props: { label: { type: String, default: '' }, disabled: Boolean, loading: Boolean },
  emits: ['click'],
  template: '<div v-bind="$attrs" role="button" :data-disabled="disabled ? \'true\' : undefined" :data-loading="loading" @click="$emit(\'click\')">{{ label }}</div>',
})
const ConfirmDialogStub = defineComponent({
  props: { open: Boolean },
  emits: ['update:open', 'confirm'],
  template: '<div v-if="open" data-test="maintenance-confirm"><div role="button" @click="$emit(\'confirm\')">confirm</div></div>',
})
const NoticeStub = defineComponent({
  props: { receipt: { type: Object, default: null }, error: { type: String, default: null }, message: { type: String, default: null } },
  emits: ['refresh'],
  template: '<div v-if="receipt || error" data-test="maintenance-status" :data-status="receipt?.status"><span v-if="message">{{ message }}</span><span v-if="error" data-test="maintenance-error">{{ error }}</span></div>',
})

function receipt(status: string) {
  return { id: 'operation-1', kind: 'admin_maintenance', status }
}

function mountTrigger() {
  return mount(MaintenanceTrigger, { global: { stubs: { UButton: ButtonStub, ConfirmDialog: ConfirmDialogStub, OperationStatusNotice: NoticeStub } } })
}

describe('MaintenanceTrigger', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mocks.queue.mockReset()
    mocks.getOperation.mockReset()
  })

  afterEach(() => vi.useRealTimers())

  it('requires confirmation, polls status, and emits completion after success', async () => {
    mocks.queue.mockResolvedValue(receipt('queued'))
    mocks.getOperation.mockResolvedValue(receipt('succeeded'))
    const wrapper = mountTrigger()

    await wrapper.get('.maintenance-trigger > *').trigger('click')
    expect(mocks.queue).not.toHaveBeenCalled()
    await wrapper.get('[data-test="maintenance-confirm"] [role="button"]').trigger('click')
    await flushPromises()

    expect(mocks.queue).toHaveBeenCalledOnce()
    const trigger = wrapper.get('.maintenance-trigger > *')
    expect(trigger.attributes('disabled') ?? trigger.attributes('data-disabled')).not.toBeUndefined()
    expect(wrapper.get('[data-test="maintenance-status"]').attributes('data-status')).toBe('queued')

    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()
    expect(mocks.getOperation).toHaveBeenCalledWith('operation-1')
    expect(wrapper.get('[data-test="maintenance-status"]').attributes('data-status')).toBe('succeeded')
    expect(wrapper.emitted('completed')).toHaveLength(1)
  })

  it('shows queue errors and keeps the trigger enabled for retry', async () => {
    mocks.queue.mockRejectedValue(new Error('queue unavailable'))
    const wrapper = mountTrigger()

    await wrapper.get('.maintenance-trigger > *').trigger('click')
    await wrapper.get('[data-test="maintenance-confirm"] [role="button"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('[data-test="maintenance-error"]')).toHaveLength(1)
    const trigger = wrapper.get('.maintenance-trigger > *')
    expect(trigger.attributes('disabled') ?? trigger.attributes('data-disabled')).toBeUndefined()
    expect(wrapper.emitted('completed')).toBeUndefined()
  })

  it('keeps review-required maintenance blocked instead of offering another confirmation', async () => {
    mocks.queue.mockResolvedValue(receipt('pending_review'))
    const wrapper = mountTrigger()
    await wrapper.get('.maintenance-trigger > *').trigger('click')
    await wrapper.get('[data-test="maintenance-confirm"] [role="button"]').trigger('click')
    await flushPromises()
    const trigger = wrapper.get('.maintenance-trigger > *')
    expect(trigger.attributes('disabled') ?? trigger.attributes('data-disabled')).not.toBeUndefined()
    await trigger.trigger('click')
    expect(wrapper.find('[data-test="maintenance-confirm"]').exists()).toBe(false)
    expect(mocks.queue).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
})
