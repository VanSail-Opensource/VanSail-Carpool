import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { embyConnectionApi, type EmbyConnection } from '@/api/embyConnection'
import AdminEmbyConnection from './AdminEmbyConnection.vue'

afterEach(() => vi.restoreAllMocks())

describe('administrator connection secret lifetime', () => {
  it('does not report success or permit another write after the workspace leaves', async () => {
    const current: EmbyConnection = { version: 'a'.repeat(64), urlConfigured: true, tokenConfigured: true }
    vi.spyOn(embyConnectionApi, 'get').mockResolvedValue(current)
    let finish!: (value: EmbyConnection) => void
    const save = vi.spyOn(embyConnectionApi, 'save').mockReturnValue(new Promise(resolve => { finish = resolve }))
    const wrapper = shallowMount(AdminEmbyConnection, {
      global: { renderStubDefaultSlot: true },
    })
    await flushPromises()
    wrapper.findAllComponents({ name: 'Input' })[1]!.vm.$emit('update:modelValue', 'reviewed-secret-token')
    const pending = wrapper.vm.save()
    expect(save).toHaveBeenCalledExactlyOnceWith({ version: current.version, baseURL: '', apiToken: 'reviewed-secret-token' })
    wrapper.unmount()
    finish({ ...current, version: 'b'.repeat(64) })
    expect(await pending).toBe(false)
    expect(await wrapper.vm.save()).toBe(false)
    expect(save).toHaveBeenCalledOnce()
  })
})
