/* eslint-disable vue/one-component-per-file */

import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref, shallowRef } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import XNodeFirewallFields from './XNodeFirewallFields.vue'
import type { FirewallPolicy } from './firewallPorts'

vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const control = (tag: 'input' | 'textarea') => defineComponent({
  props: { modelValue: { type: String, default: '' } }, emits: ['update:modelValue'],
  setup: (props, { emit }) => () => h(tag, { value: props.modelValue, onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value) }),
})
const stubs = {
  FormField: { props: { label: String, error: String }, template: '<label :data-label="label" :data-error="error"><slot /></label>' },
  Input: control('input'), Textarea: control('textarea'), Select: { template: '<span />' },
  Button: defineComponent({ setup: (_, { slots }) => () => h('button', { type: 'button' }, slots.default?.()) }),
}

function editor(policy: FirewallPolicy) {
  const model = ref(policy), valid = shallowRef(true)
  const wrapper = mount(defineComponent({ setup: () => () => h(XNodeFirewallFields, {
    modelValue: model.value, 'onUpdate:modelValue': (value: FirewallPolicy) => { model.value = value }, onValid: (value: boolean) => { valid.value = value },
  }) }), { global: { stubs } })
  return { model, valid, wrapper }
}

describe('firewall draft validation', () => {
  it('reads saved singletons and ranges while preserving invalid text across unrelated edits', async () => {
    const { wrapper, model, valid } = editor({ enabled: true, protect: false, tcp_ports: [22, 443], tcp_port_ranges: [{ from: 443, to: 445 }] })
    const ports = wrapper.get('[data-label="nodeForms.tcp_ports"] textarea')
    expect((ports.element as HTMLTextAreaElement).value).toBe('22, 443-445')
    await ports.setValue('65536-')
    expect(valid.value).toBe(false)
    await wrapper.get('[data-label="nodeForms.allow_prefixes"] textarea').setValue('192.0.2.0/24')
    expect((ports.element as HTMLTextAreaElement).value).toBe('65536-')
    expect(valid.value).toBe(false)
    expect(model.value.tcp_ports).toEqual([22, 443])
    await ports.setValue('443 8443, 20000 - 30000, 29999-31000')
    expect(valid.value).toBe(true)
    expect(model.value).toMatchObject({ tcp_ports: [443, 8443], tcp_port_ranges: [{ from: 20000, to: 31000 }] })
    wrapper.unmount()
  })

  it('keeps partial forwarding edits local and blocks saving until all fields are valid', async () => {
    const { wrapper, model, valid } = editor({ enabled: true, protect: false, forward: [{ listen_ip: '', port: 1000, port_end: 1009, target_ip: '192.0.2.10', target_port: 2000, target_port_end: 2009, network: 'udp' }] })
    await wrapper.get('[data-label="nodeForms.sourcePorts"] input').setValue('1000-1019')
    expect(valid.value).toBe(false)
    expect(model.value.forward?.[0]?.port_end).toBe(1009)
    await wrapper.get('[data-label="nodeForms.targetPorts"] input').setValue('2000-')
    await wrapper.get('[data-label="nodeForms.tcp_ports"] textarea').setValue('443')
    expect((wrapper.get('[data-label="nodeForms.targetPorts"] input').element as HTMLInputElement).value).toBe('2000-')
    expect(valid.value).toBe(false)
    await wrapper.get('[data-label="nodeForms.targetPorts"] input').setValue('2000-2019')
    expect(valid.value).toBe(true)
    expect(model.value.forward?.[0]).toMatchObject({ port: 1000, port_end: 1019, target_port: 2000, target_port_end: 2019 })
    await wrapper.get('[aria-label="nodeForms.addForward"]').trigger('click')
    expect(valid.value).toBe(false)
    await wrapper.findAll('[aria-label="common.delete"]')[1]!.trigger('click')
    expect(valid.value).toBe(true)
    expect(model.value.forward).toHaveLength(1)
    wrapper.unmount()
  })

  it('reloads external policy changes and keeps malformed CIDR input visible', async () => {
    const { wrapper, model, valid } = editor({ enabled: false, protect: false })
    model.value = { enabled: true, protect: false, udp_port_ranges: [{ from: 20000, to: 30000 }] }
    await nextTick()
    expect((wrapper.get('[data-label="nodeForms.udp_ports"] textarea').element as HTMLTextAreaElement).value).toBe('20000-30000')
    await wrapper.get('[data-label="nodeForms.allow_prefixes"] textarea').setValue('192.0.2.1/no')
    expect(valid.value).toBe(false)
    expect(wrapper.get('[data-label="nodeForms.allow_prefixes"]').attributes('data-error')).toBe('nodeForms.prefixListInvalid')
    wrapper.unmount()
  })
})
