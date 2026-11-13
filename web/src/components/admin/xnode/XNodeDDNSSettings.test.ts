import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { XNodeSettings } from '@/api/xnode'
import XNodeDDNSSettings from './XNodeDDNSSettings.vue'
import XNodeDDNSJob from './XNodeDDNSJob.vue'

function editor() {
  const model = ref<XNodeSettings>({
    ddns: ['first', 'second'].map(id => ({ id, provider: 'provider', zone: 'example.com', name: `${id}.example.com`, ipv4: 'auto', interface: 'eth0' })),
    ddnsBindings: { first: { ipv4HostIds: ['first-host'] } },
  })
  const wrapper = mount(defineComponent({ setup: () => () => h(XNodeDDNSSettings, {
    modelValue: model.value, 'onUpdate:modelValue': (value: XNodeSettings) => { model.value = value },
  }) }), { global: { stubs: { XNodeDDNSJob: true, Button: true } } })
  return { model, wrapper }
}

describe('DDNS row and endpoint ownership', () => {
  it('does not transfer a removed job binding to the following unbound job', async () => {
    const { model, wrapper } = editor()
    wrapper.findAllComponents(XNodeDDNSJob)[0]!.vm.$emit('remove')
    await wrapper.vm.$nextTick()
    expect(model.value.ddns?.map(job => job.id)).toEqual(['second'])
    expect(model.value.ddnsBindings).toEqual({})
    wrapper.unmount()
  })

  it('keeps the same editor and its own binding through a temporary duplicate name', async () => {
    const { model, wrapper } = editor()
    model.value.ddnsBindings!.second = { ipv4HostIds: ['second-host'] }
    await wrapper.vm.$nextTick()
    const firstEditor = wrapper.findAllComponents(XNodeDDNSJob)[0]!.vm
    model.value.ddns![0]!.id = 'second'
    await wrapper.vm.$nextTick()
    expect(wrapper.findAllComponents(XNodeDDNSJob)[0]!.vm).toBe(firstEditor)
    expect(wrapper.findAllComponents(XNodeDDNSJob)[0]!.props('binding')).toEqual({ ipv4HostIds: ['first-host'] })
    model.value.ddns![0]!.id = 'renamed'
    await wrapper.vm.$nextTick()
    expect(wrapper.findAllComponents(XNodeDDNSJob)[0]!.vm).toBe(firstEditor)
    expect(model.value.ddnsBindings).toEqual({ renamed: { ipv4HostIds: ['first-host'] }, second: { ipv4HostIds: ['second-host'] } })
    wrapper.unmount()
  })
})
