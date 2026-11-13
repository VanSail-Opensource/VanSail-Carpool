import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UForm from '@nuxt/ui/runtime/components/Form.vue'
import { firewallApi } from '@/api/firewall'
import { ApiError } from '@/api/http'
import XNodeFirewallGrant from './XNodeFirewallGrant.vue'

describe('firewall grant retry identity', () => {
  it('retains an ambiguous key when whitespace changes leave the submitted prefix unchanged', async () => {
    const grant = vi.spyOn(firewallApi, 'grant').mockRejectedValue(new ApiError(503, { code: 'TEMPORARY_FAILURE', message: '' }))
    const wrapper = shallowMount(XNodeFirewallGrant, { global: { renderStubDefaultSlot: true } })
    const prefix = wrapper.findAllComponents({ name: 'Input' })[0]!
    prefix.vm.$emit('update:modelValue', ' 192.0.2.4 ')
    await wrapper.vm.$nextTick()
    wrapper.getComponent(UForm).vm.$emit('submit')
    await flushPromises()
    prefix.vm.$emit('update:modelValue', '192.0.2.4')
    await wrapper.vm.$nextTick()
    wrapper.getComponent(UForm).vm.$emit('submit')
    await flushPromises()
    expect(grant).toHaveBeenCalledTimes(2)
    expect(grant.mock.calls[0]![0]).toEqual({ prefix: '192.0.2.4', hours: 24, reason: '' })
    expect(grant.mock.calls[1]![1]).toBe(grant.mock.calls[0]![1])
    wrapper.unmount()
    grant.mockRestore()
  })
})
