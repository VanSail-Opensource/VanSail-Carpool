import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import UApp from '@nuxt/ui/runtime/components/App.vue'
import APITokenForm from './APITokenForm.vue'

describe('grouped API permissions', () => {
  it('gives every checkbox its own native label and keeps other group selections', async () => {
    const wrapper = mount(defineComponent({ setup: () => () => h(UApp, null, { default: () => h(APITokenForm, { busy: false }) }) }))
    const controls = wrapper.findAll('[role="checkbox"]')
    const ids = controls.map(control => control.attributes('id'))
    expect(new Set(ids).size).toBe(6)
    for (const id of ids) expect(wrapper.find(`label[for="${id}"]`).text()).toBeTruthy()
    await wrapper.get('input[name="name"]').setValue('Accounting monitor')
    await wrapper.get('#api-token-scope-admin-read').trigger('click')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    const body = wrapper.findComponent(APITokenForm).emitted('create')?.[0]?.[0] as { scopes: string[] }
    expect(body.scopes).toEqual(expect.arrayContaining(['admin:read', 'xnode:write', 'migration:write']))
    wrapper.unmount()
  })
})
