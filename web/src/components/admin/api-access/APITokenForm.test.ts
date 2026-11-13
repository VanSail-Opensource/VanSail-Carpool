/* eslint-disable vue/one-component-per-file -- Each case mounts an isolated Nuxt UI app harness. */
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import UApp from '@nuxt/ui/runtime/components/App.vue'
import APITokenForm from './APITokenForm.vue'
import { t } from '@/i18n'

describe('grouped API permissions', () => {
  it('gives every checkbox its own native label and keeps other group selections', async () => {
    const wrapper = mount(defineComponent({ setup: () => () => h(UApp, null, { default: () => h(APITokenForm, { busy: false }) }) }))
    const controls = wrapper.findAll('[role="checkbox"]')
    const ids = controls.map(control => control.attributes('id'))
    expect(new Set(ids).size).toBe(4)
    for (const id of ids) expect(wrapper.find(`label[for="${id}"]`).text()).toBeTruthy()
    await wrapper.get('input[name="name"]').setValue('Accounting monitor')
    await wrapper.get('#api-token-scope-xnode-write').trigger('click')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    const body = wrapper.findComponent(APITokenForm).emitted('create')?.[0]?.[0] as { scopes: string[] }
    expect(body.scopes).toEqual(['admin:read', 'xnode:write'])
    wrapper.unmount()
  })

  it.each([
    ['apiTokens.readOnly', ['admin:read', 'xnode:read']],
    ['apiTokens.fullAccess', ['admin:write', 'xnode:write']],
  ] as const)('selects independent permission families for %s', async (label, scopes) => {
    const wrapper = mount(defineComponent({ setup: () => () => h(UApp, null, { default: () => h(APITokenForm, { busy: false }) }) }))
    await wrapper.get('input[name="name"]').setValue('Operations')
    await wrapper.findAll('button').find(button => button.text() === t(label))!.trigger('click')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect((wrapper.findComponent(APITokenForm).emitted('create')?.[0]?.[0] as { scopes: string[] }).scopes).toEqual(scopes)
    wrapper.unmount()
  })
})
